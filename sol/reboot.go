package sol

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

type serverState struct {
	inOS       bool      // true if we've seen OS output
	lastReboot time.Time // last time we detected a reboot
}

type RebootDetector struct {
	biosPatterns []*regexp.Regexp
	osPatterns   []*regexp.Regexp
	states       map[string]*serverState
	cooldown     time.Duration
	mu           sync.Mutex
}

func NewRebootDetector(patterns []string) *RebootDetector {
	rd := &RebootDetector{
		biosPatterns: make([]*regexp.Regexp, 0),
		osPatterns:   make([]*regexp.Regexp, 0),
		states:       make(map[string]*serverState),
		cooldown:     2 * time.Minute,
	}

	// BIOS/POST patterns - indicate we're in boot process
	biosPatterns := []string{
		`American Megatrends`,
		`Press <DEL> to run Setup`,
		`Press DEL to run Setup`,
		`BIOS Date:`,
		`Supermicro`,
		`Intel\(R\) Boot Agent`,
		`PXE-`,
		`PXE->`,
		`PXELINUX`,
		`iPXE initialising`,
		`iPXE \d+\.\d+`,
		`Open Source Network Boot Firmware`,
		`Booting baremetalservices`,
		`UNDI code segment`,
		`CLIENT MAC ADDR:`,
		`free base memory after PXE`,
	}

	// OS patterns - indicate the OS is running
	osPatterns := []string{
		`\[\s*\d+\.\d+\]`,           // Linux dmesg timestamps like [    0.000000]
		`Linux version \d+\.\d+`,    // Kernel version line
		`login:`,                    // Login prompt
		`#\s*$`,                     // Root shell prompt
		`\$\s*$`,                    // User shell prompt
		`systemd\[`,                 // Systemd messages
		`Starting.*\.\.\.`,          // Service starting messages
		`Started `,                  // Service started messages
		`eth\d+:.*link`,             // Network link messages
		`NTP sync`,                  // NTP messages
	}

	// Add user-configured patterns to BIOS patterns
	allBiosPatterns := append(biosPatterns, patterns...)

	for _, p := range allBiosPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err == nil {
			rd.biosPatterns = append(rd.biosPatterns, re)
		}
	}

	for _, p := range osPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err == nil {
			rd.osPatterns = append(rd.osPatterns, re)
		}
	}

	return rd
}

func (rd *RebootDetector) Check(serverName, text string) bool {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	state, exists := rd.states[serverName]
	if !exists {
		state = &serverState{inOS: false}
		rd.states[serverName] = state
	}

	// Check cooldown
	if time.Since(state.lastReboot) < rd.cooldown {
		// Still in cooldown, but update OS state if we see OS patterns
		if rd.matchesOS(text) {
			state.inOS = true
		}
		return false
	}

	// Check if we see OS patterns - mark that OS is running
	if rd.matchesOS(text) {
		state.inOS = true
		return false
	}

	// Check if we see BIOS patterns
	if rd.matchesBIOS(text) {
		// Only trigger if we were previously in OS state
		// This means we transitioned from OS -> BIOS = reboot
		if state.inOS {
			state.inOS = false
			state.lastReboot = time.Now()
			return true
		}
		// If we weren't in OS state, we're still in boot process
		// Don't trigger another rotation
		return false
	}

	return false
}

func (rd *RebootDetector) matchesBIOS(text string) bool {
	for _, p := range rd.biosPatterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

func (rd *RebootDetector) matchesOS(text string) bool {
	// Also check for common OS indicators without regex
	lowerText := strings.ToLower(text)
	simplePatterns := []string{
		"welcome to",
		"kernel:",
		"last login:",
		"no mail",
		"/home/",
		"/root/",
		"apt ",
		"yum ",
		"apk ",
		"docker",
		"kubelet",
	}
	for _, p := range simplePatterns {
		if strings.Contains(lowerText, p) {
			return true
		}
	}

	for _, p := range rd.osPatterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// MarkOSRunning can be called externally to mark that the OS is running
// (e.g., after loading existing log content)
func (rd *RebootDetector) MarkOSRunning(serverName string) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	state, exists := rd.states[serverName]
	if !exists {
		state = &serverState{}
		rd.states[serverName] = state
	}
	state.inOS = true
}
