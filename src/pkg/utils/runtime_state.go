package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	RuntimeDir        = "/run/discord-bypass"
	RuntimeStatusFile = "/run/discord-bypass/status.json"
	FallbackStatus    = "/run/discord-bypass-status.json"
)

// RuntimeStatus encapsulates the live daemon state and packet interception counters.
// It is written with mode 0644 so unprivileged users can inspect status and diagnostics.
type RuntimeStatus struct {
	PID            int       `json:"pid"`
	StartTime      time.Time `json:"start_time"`
	UpdateTime     time.Time `json:"update_time"`
	ServiceActive  bool      `json:"service_active"`
	BackendName    string    `json:"backend_name"`
	BackendRunning bool      `json:"backend_running"`
	BackendPID     int       `json:"backend_pid"`
	ActiveStrategy string    `json:"active_strategy"`
	StrategyName   string    `json:"strategy_name"`
	FirewallDriver string    `json:"firewall_driver"`
	FirewallActive bool      `json:"firewall_active"`
	Packets        uint64    `json:"packets"`
	Bytes          uint64    `json:"bytes"`
	DNSMode        string    `json:"dns_mode"`
	DoHProvider    string    `json:"doh_provider"`
	SyncHosts      bool      `json:"sync_hosts"`
	TargetDomains  int       `json:"target_domains"`
}

// WriteRuntimeStatus atomically serializes the runtime status to /run/discord-bypass/status.json
func WriteRuntimeStatus(status *RuntimeStatus) error {
	if status == nil {
		return fmt.Errorf("status is nil")
	}
	status.UpdateTime = time.Now()

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal runtime status: %w", err)
	}

	targetPath := RuntimeStatusFile
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		targetPath = FallbackStatus
	}

	tmpFile := targetPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		// Fallback to direct write or root fallback
		if errFallback := os.WriteFile(FallbackStatus, data, 0644); errFallback == nil {
			return nil
		}
		return fmt.Errorf("failed to write runtime status: %w", err)
	}

	if err := os.Rename(tmpFile, targetPath); err != nil {
		_ = os.Remove(tmpFile)
		_ = os.WriteFile(targetPath, data, 0644)
	}
	return nil
}

// ReadRuntimeStatus retrieves the latest daemon status from the state file.
func ReadRuntimeStatus() (*RuntimeStatus, error) {
	paths := []string{RuntimeStatusFile, FallbackStatus}
	var data []byte
	var err error

	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	var status RuntimeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to parse runtime status: %w", err)
	}

	// Verify status is not excessively stale (e.g. from an unclean shutdown > 30s ago)
	if time.Since(status.UpdateTime) > 30*time.Second {
		status.FirewallActive = false
		status.BackendRunning = false
	}

	return &status, nil
}

// RemoveRuntimeStatus removes the runtime status file upon daemon exit.
func RemoveRuntimeStatus() {
	_ = os.Remove(RuntimeStatusFile)
	_ = os.Remove(FallbackStatus)
}
