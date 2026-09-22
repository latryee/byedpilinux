package dns

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"discord-bypass/src/pkg/utils"
)

const (
	HostsTagBegin = "# --- BEGIN DISCORD-BYPASS (DO NOT EDIT) ---"
	HostsTagEnd   = "# --- END DISCORD-BYPASS ---"
	HostsFilePath = "/etc/hosts"
)

var hostsMu sync.Mutex

// ApplyDiscordHosts safely inserts or updates Discord domain IP mappings in /etc/hosts
func ApplyDiscordHosts(domainToIP map[string]net.IP) error {
	hostsMu.Lock()
	defer hostsMu.Unlock()

	data, err := os.ReadFile(HostsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", HostsFilePath, err)
	}

	cleaned := removeTaggedBlock(string(data))

	var newBlock bytes.Buffer
	newBlock.WriteString(HostsTagBegin + "\n")
	newBlock.WriteString("# Automatically managed by discord-bypass daemon\n")
	newBlock.WriteString("# Overrides ISP DNS poisoning for Discord endpoints\n")

	for domain, ip := range domainToIP {
		if ip == nil || IsSinkholeIP(ip) {
			continue
		}
		newBlock.WriteString(fmt.Sprintf("%-16s %s\n", ip.String(), domain))
	}
	newBlock.WriteString(HostsTagEnd + "\n")

	finalContent := strings.TrimRight(cleaned, "\n") + "\n\n" + newBlock.String()

	// Write atomically using temporary file in /etc
	tmpPath := HostsFilePath + ".tmp.discord-bypass"
	if err := os.WriteFile(tmpPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write tmp hosts file: %w", err)
	}

	if err := os.Rename(tmpPath, HostsFilePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace %s: %w", HostsFilePath, err)
	}

	utils.Info("Updated %s with %d secure Discord IP mappings", HostsFilePath, len(domainToIP))
	return nil
}

// RemoveDiscordHosts removes the discord-bypass block from /etc/hosts
func RemoveDiscordHosts() error {
	hostsMu.Lock()
	defer hostsMu.Unlock()

	data, err := os.ReadFile(HostsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cleaned := removeTaggedBlock(string(data))
	if cleaned == string(data) {
		// Nothing to remove
		return nil
	}

	tmpPath := HostsFilePath + ".tmp.discord-bypass"
	if err := os.WriteFile(tmpPath, []byte(cleaned), 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, HostsFilePath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	utils.Info("Cleanly removed discord-bypass entries from %s", HostsFilePath)
	return nil
}

func removeTaggedBlock(content string) string {
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	inBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == HostsTagBegin {
			inBlock = true
			continue
		}
		if strings.TrimSpace(line) == HostsTagEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n") + "\n"
}

// HasDiscordHosts checks if /etc/hosts contains the discord-bypass managed block
func HasDiscordHosts() bool {
	data, err := os.ReadFile(HostsFilePath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), HostsTagBegin)
}
