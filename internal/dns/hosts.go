package dns

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const (
	hostsFile   = "/etc/hosts"
	markerStart = "# WTM-START"
	markerEnd   = "# WTM-END"
)

// HostsManager manages /etc/hosts entries
type HostsManager struct {
	mu sync.Mutex
}

// NewHostsManager creates a new hosts manager
func NewHostsManager() *HostsManager {
	return &HostsManager{}
}

// Entry represents a hosts file entry
type Entry struct {
	IP     string
	Domain string
}

// Sync updates /etc/hosts to match the desired domains
func (m *HostsManager) Sync(domains []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read current hosts file
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	// Remove existing WTM entries
	newContent := m.removeMarkedSection(string(content))

	// Add new entries if any
	if len(domains) > 0 {
		newContent = m.addMarkedSection(newContent, domains)
	}

	// Write back using sudo
	return m.writeHostsFile(newContent)
}

// Add adds domains to /etc/hosts
func (m *HostsManager) Add(domains ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	// Get existing domains
	existing := m.getMarkedDomains(string(content))

	// Add new domains
	allDomains := append(existing, domains...)
	allDomains = uniqueStrings(allDomains)

	// Remove old section and add new
	newContent := m.removeMarkedSection(string(content))
	newContent = m.addMarkedSection(newContent, allDomains)

	return m.writeHostsFile(newContent)
}

// Remove removes domains from /etc/hosts
func (m *HostsManager) Remove(domains ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("reading hosts file: %w", err)
	}

	// Get existing domains
	existing := m.getMarkedDomains(string(content))

	// Remove specified domains
	remaining := make([]string, 0)
	for _, d := range existing {
		remove := false
		for _, r := range domains {
			if d == r {
				remove = true
				break
			}
		}
		if !remove {
			remaining = append(remaining, d)
		}
	}

	// Remove old section and add new
	newContent := m.removeMarkedSection(string(content))
	if len(remaining) > 0 {
		newContent = m.addMarkedSection(newContent, remaining)
	}

	return m.writeHostsFile(newContent)
}

// List returns all WTM-managed domains
func (m *HostsManager) List() ([]Entry, error) {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return nil, fmt.Errorf("reading hosts file: %w", err)
	}

	domains := m.getMarkedDomains(string(content))
	entries := make([]Entry, len(domains))
	for i, d := range domains {
		entries[i] = Entry{IP: "127.0.0.1", Domain: d}
	}
	return entries, nil
}

// removeMarkedSection removes the WTM section from content
func (m *HostsManager) removeMarkedSection(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	inSection := false

	for _, line := range lines {
		if strings.TrimSpace(line) == markerStart {
			inSection = true
			continue
		}
		if strings.TrimSpace(line) == markerEnd {
			inSection = false
			continue
		}
		if !inSection {
			result = append(result, line)
		}
	}

	// Trim trailing empty lines
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return strings.Join(result, "\n")
}

// addMarkedSection adds a new WTM section
func (m *HostsManager) addMarkedSection(content string, domains []string) string {
	var section strings.Builder
	section.WriteString("\n\n")
	section.WriteString(markerStart)
	section.WriteString("\n")
	section.WriteString("# Managed by worktree-manager - do not edit manually\n")

	for _, domain := range domains {
		section.WriteString(fmt.Sprintf("127.0.0.1 %s\n", domain))
	}

	section.WriteString(markerEnd)
	section.WriteString("\n")

	return content + section.String()
}

// getMarkedDomains extracts domains from the WTM section
func (m *HostsManager) getMarkedDomains(content string) []string {
	lines := strings.Split(content, "\n")
	domains := make([]string, 0)
	inSection := false

	for _, line := range lines {
		if strings.TrimSpace(line) == markerStart {
			inSection = true
			continue
		}
		if strings.TrimSpace(line) == markerEnd {
			inSection = false
			continue
		}
		if inSection {
			parts := strings.Fields(line)
			if len(parts) >= 2 && !strings.HasPrefix(parts[0], "#") {
				domains = append(domains, parts[1])
			}
		}
	}

	return domains
}

// writeHostsFile writes to /etc/hosts using sudo
func (m *HostsManager) writeHostsFile(content string) error {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "wtm-hosts-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	// Use sudo to copy the file
	cmd := exec.Command("sudo", "cp", tmpFile.Name(), hostsFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updating hosts file (sudo required): %w", err)
	}

	return nil
}

// uniqueStrings returns unique strings
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(strs))
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ReadHostsFile reads and parses /etc/hosts
func ReadHostsFile() ([]Entry, error) {
	file, err := os.Open(hostsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			for _, domain := range parts[1:] {
				entries = append(entries, Entry{IP: parts[0], Domain: domain})
			}
		}
	}

	return entries, scanner.Err()
}
