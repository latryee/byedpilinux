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
		// If /etc directory is mounted read-only (e.g. systemd ProtectSystem=full), try direct in-place write
		if wErr := os.WriteFile(HostsFilePath, []byte(finalContent), 0644); wErr == nil {
			utils.Info("Updated %s directly with %d secure Discord IP mappings", HostsFilePath, len(domainToIP))
			return nil
		}
		// Non-fatal: log informative note and continue (split-DNS handles resolution)
		utils.Debug("Cannot write %s (%v); skipping hosts sync (split-DNS remains active)", HostsFilePath, err)
		return nil
	}

	if err := os.Rename(tmpPath, HostsFilePath); err != nil {
		_ = os.Remove(tmpPath)
		// If rename fails (e.g. read-only directory), try in-place write
		if wErr := os.WriteFile(HostsFilePath, []byte(finalContent), 0644); wErr == nil {
			utils.Info("Updated %s directly with %d secure Discord IP mappings", HostsFilePath, len(domainToIP))
			return nil
		}
		utils.Debug("Cannot replace %s (%v); skipping hosts sync (split-DNS remains active)", HostsFilePath, err)
		return nil
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
		return nil
	}

	cleaned := removeTaggedBlock(string(data))
	if cleaned == string(data) {
		// Nothing to remove
		return nil
	}

	tmpPath := HostsFilePath + ".tmp.discord-bypass"
	if err := os.WriteFile(tmpPath, []byte(cleaned), 0644); err != nil {
		if wErr := os.WriteFile(HostsFilePath, []byte(cleaned), 0644); wErr == nil {
			utils.Info("Cleanly removed discord-bypass entries from %s", HostsFilePath)
			return nil
		}
		utils.Debug("Cannot remove entries from %s: %v (skipping)", HostsFilePath, err)
		return nil
	}

	if err := os.Rename(tmpPath, HostsFilePath); err != nil {
		_ = os.Remove(tmpPath)
		if wErr := os.WriteFile(HostsFilePath, []byte(cleaned), 0644); wErr == nil {
			utils.Info("Cleanly removed discord-bypass entries from %s", HostsFilePath)
			return nil
		}
		utils.Debug("Cannot replace %s on cleanup: %v (skipping)", HostsFilePath, err)
		return nil
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
