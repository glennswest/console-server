package sol

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Session struct {
	ServerName  string
	IP          string
	Connected   bool
	LastError   string
	cancel      context.CancelFunc
	subscribers map[chan []byte]struct{}
	subMu       sync.RWMutex
}

type Manager struct {
	username       string
	password       string
	sessions       map[string]*Session
	mu             sync.RWMutex
	logWriter      LogWriter
	rebootDetector *RebootDetector
	analytics      *Analytics
}

type LogWriter interface {
	Write(serverName string, data []byte) error
	Rotate(serverName string) error
	CanRotate(serverName string) bool
}

func NewManager(username, password string, logWriter LogWriter, rebootDetector *RebootDetector, dataPath string) *Manager {
	return &Manager{
		username:       username,
		password:       password,
		sessions:       make(map[string]*Session),
		logWriter:      logWriter,
		rebootDetector: rebootDetector,
		analytics:      NewAnalytics(dataPath),
	}
}

func (m *Manager) GetAnalytics(serverName string) *ServerAnalytics {
	return m.analytics.GetServerAnalytics(serverName)
}

func (m *Manager) GetAllAnalytics() map[string]*ServerAnalytics {
	return m.analytics.GetAllAnalytics()
}

func (m *Manager) StartSession(serverName, ip string) {
	m.mu.Lock()
	if existing, exists := m.sessions[serverName]; exists {
		if existing.cancel != nil {
			existing.cancel()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{
		ServerName:  serverName,
		IP:          ip,
		Connected:   false,
		cancel:      cancel,
		subscribers: make(map[chan []byte]struct{}),
	}
	m.sessions[serverName] = session
	m.mu.Unlock()

	go m.runSession(ctx, session)
}

func (m *Manager) StopSession(serverName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[serverName]; exists {
		if session.cancel != nil {
			session.cancel()
		}
		// Close all subscriber channels
		session.subMu.Lock()
		for ch := range session.subscribers {
			close(ch)
		}
		session.subscribers = nil
		session.subMu.Unlock()
		delete(m.sessions, serverName)
	}
}

func (m *Manager) GetSession(serverName string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[serverName]
}

func (m *Manager) GetSessions() map[string]*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Session)
	for k, v := range m.sessions {
		result[k] = v
	}
	return result
}

func (m *Manager) Subscribe(serverName string) (<-chan []byte, func()) {
	m.mu.RLock()
	session, exists := m.sessions[serverName]
	m.mu.RUnlock()

	if !exists {
		return nil, nil
	}

	ch := make(chan []byte, 100)

	session.subMu.Lock()
	session.subscribers[ch] = struct{}{}
	session.subMu.Unlock()

	unsubscribe := func() {
		session.subMu.Lock()
		delete(session.subscribers, ch)
		session.subMu.Unlock()
	}

	return ch, unsubscribe
}

func (m *Manager) runSession(ctx context.Context, session *Session) {
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Infof("Connecting SOL to %s (%s)", session.ServerName, session.IP)

		err := m.connectSOL(ctx, session)
		if err != nil {
			session.Connected = false
			session.LastError = err.Error()
			log.Errorf("SOL connection failed for %s: %v", session.ServerName, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			// Exponential backoff with max 60s
			backoff = backoff * 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
		}
	}
}

func (m *Manager) connectSOL(ctx context.Context, session *Session) error {
	// SOL disabled - TTY handling on arm64 container causes hangs
	// TODO: Implement native IPMI SOL using goipmi library
	session.Connected = false
	session.LastError = "SOL disabled - arm64 TTY issues"
	return fmt.Errorf("SOL disabled")
}
