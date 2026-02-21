package discovery

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	IP       string
	Hostname string
	Online   bool
	MAC      string
}

// PXEHost represents a host from the pxemanager API
type PXEHost struct {
	ID           int64  `json:"id"`
	MAC          string `json:"mac"`
	Hostname     string `json:"hostname"`
	CurrentImage string `json:"current_image"`
	IPMIIP       string `json:"ipmi_ip"`
	IPMIUsername string `json:"ipmi_username"`
	IPMIPassword string `json:"ipmi_password"`
}

type Scanner struct {
	servers    map[string]*Server
	mu         sync.RWMutex
	onChange   func(servers map[string]*Server)
	pxeURL     string
	httpClient *http.Client
}

func NewScanner(pxeURL string) *Scanner {
	return &Scanner{
		servers:    make(map[string]*Server),
		pxeURL:     pxeURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Scanner) AddServer(name, host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Resolve hostname to IP
	ip := host
	if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
		ip = addrs[0]
	}

	s.servers[name] = &Server{
		IP:       ip,
		Hostname: name,
		Online:   true,
	}

	log.Infof("Added server: %s (%s -> %s)", name, host, ip)
}

func (s *Scanner) OnChange(fn func(servers map[string]*Server)) {
	s.onChange = fn
}

func (s *Scanner) GetServers() map[string]*Server {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*Server)
	for k, v := range s.servers {
		result[k] = v
	}
	return result
}

// Refresh triggers an immediate fetch from pxemanager
func (s *Scanner) Refresh() {
	s.fetchFromPXE()
}

func (s *Scanner) Run(ctx context.Context) {
	// Initial fetch from pxemanager
	s.fetchFromPXE()

	// Trigger initial onChange
	if s.onChange != nil {
		s.onChange(s.GetServers())
	}

	// Periodic refresh from pxemanager
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchFromPXE()
		}
	}
}

func (s *Scanner) fetchFromPXE() {
	if s.pxeURL == "" {
		return
	}

	resp, err := s.httpClient.Get(s.pxeURL + "/api/hosts")
	if err != nil {
		log.Warnf("Failed to fetch hosts from pxemanager: %v", err)
		return
	}
	defer resp.Body.Close()

	var hosts []PXEHost
	if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
		log.Warnf("Failed to decode pxemanager response: %v", err)
		return
	}

	s.mu.Lock()

	knownIPs := make(map[string]string) // IP -> name
	for name, srv := range s.servers {
		knownIPs[srv.IP] = name
	}

	hasNewServers := false

	for _, h := range hosts {
		if h.IPMIIP == "" {
			continue
		}

		name := h.Hostname
		if name == "" {
			name = h.IPMIIP
		}
		// Remove domain suffix if present
		if idx := strings.Index(name, "."); idx > 0 && net.ParseIP(name) == nil {
			name = name[:idx]
		}

		if existingName, exists := knownIPs[h.IPMIIP]; exists && existingName != name {
			existing := s.servers[existingName]
			if h.MAC != "" {
				existing.MAC = h.MAC
			}
			continue
		}

		if existing, exists := s.servers[name]; exists {
			if h.MAC != "" {
				existing.MAC = h.MAC
			}
		} else {
			s.servers[name] = &Server{
				IP:       h.IPMIIP,
				Hostname: name,
				Online:   true,
				MAC:      h.MAC,
			}
			knownIPs[h.IPMIIP] = name
			log.Infof("Discovered server from pxemanager: %s (%s)", name, h.IPMIIP)
			hasNewServers = true
		}
	}

	s.mu.Unlock()

	if hasNewServers && s.onChange != nil {
		go s.onChange(s.GetServers())
	}
}
