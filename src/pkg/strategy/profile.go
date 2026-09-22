package strategy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultProfileFile = "/etc/discord-bypass/network-profile.json"
	LocalProfileFile   = "./config/network-profile.json"
)

// NetworkProfile represents local, privacy-safe state recording which strategy
// was verified on the current local network environment.
// NOTE: Strictly local. No telemetry, no public IP, no MAC address, no tracking IDs.
type NetworkProfile struct {
	NetworkName      string    `json:"network_name"`
	StrategyID       string    `json:"strategy_id"`
	StrategyName     string    `json:"strategy_name"`
	Status           string    `json:"status"`            // "verified", "unverified", "candidate"
	VerificationType string    `json:"verification_type"` // "gateway_rest", "direct", "manual"
	VerifiedAt       time.Time `json:"verified_at"`
	Source           string    `json:"source"`            // "automatic_tuning", "manual_selection", "default"
}

// GetProfilePath returns the active network profile path.
func GetProfilePath() string {
	if _, err := os.Stat("/etc/discord-bypass"); err == nil {
		return DefaultProfileFile
	}
	if _, err := os.Stat("./config"); err == nil {
		return LocalProfileFile
	}
	return DefaultProfileFile
}

// LoadNetworkProfile loads the current network profile from disk, checking system and local paths.
func LoadNetworkProfile() (*NetworkProfile, error) {
	candidates := []string{
		DefaultProfileFile,
		LocalProfileFile,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return LoadNetworkProfileFrom(c)
		}
	}
	return nil, os.ErrNotExist
}

// LoadNetworkProfileFrom loads the profile from a specific file path.
func LoadNetworkProfileFrom(path string) (*NetworkProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var prof NetworkProfile
	if err := json.Unmarshal(data, &prof); err != nil {
		return nil, fmt.Errorf("malformed network profile at %s: %w", path, err)
	}
	return &prof, nil
}

// SaveNetworkProfile saves the given profile atomically, falling back to local dir if system path is not writable.
func SaveNetworkProfile(prof *NetworkProfile) error {
	err := SaveNetworkProfileTo(DefaultProfileFile, prof)
	if err != nil && os.IsPermission(err) {
		return SaveNetworkProfileTo(LocalProfileFile, prof)
	}
	return err
}

// SaveNetworkProfileTo saves the given profile atomically to the specified path.
func SaveNetworkProfileTo(path string, prof *NetworkProfile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode network profile: %w", err)
	}
	data = append(data, '\n')

	tmpFile := filepath.Join(dir, fmt.Sprintf(".network-profile-%d.tmp", time.Now().UnixNano()))
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporary profile file %s: %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write profile data: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync profile data: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to atomically commit profile to %s: %w", path, err)
	}

	return nil
}
