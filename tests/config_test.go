package tests

import (
	"os"
	"path/filepath"
	"testing"

	"discord-bypass/src/pkg/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.General.Backend != "nfqws" {
		t.Errorf("Expected default backend nfqws, got %s", cfg.General.Backend)
	}

	if cfg.General.Strategy != "strategy_c" {
		t.Errorf("Expected default strategy strategy_c, got %s", cfg.General.Strategy)
	}

	if !cfg.General.PreferIPv4 {
		t.Errorf("Expected PreferIPv4 to be true by default")
	}

	if cfg.Firewall.QueueNum != 200 {
		t.Errorf("Expected QueueNum 200, got %d", cfg.Firewall.QueueNum)
	}

	if len(cfg.Domains) == 0 {
		t.Errorf("Expected non-empty default domains list")
	}
}

func TestLoadDomains(t *testing.T) {
	tmpDir := t.TempDir()
	domainFile := filepath.Join(tmpDir, "test_domains.txt")

	content := `# Comments should be ignored
discord.com
gateway.discord.gg # Inline comment
# Empty lines below

cdn.discordapp.com
`
	if err := os.WriteFile(domainFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed writing test domains file: %v", err)
	}

	domains, err := config.LoadDomains(domainFile)
	if err != nil {
		t.Fatalf("LoadDomains failed: %v", err)
	}

	if len(domains) != 3 {
		t.Fatalf("Expected 3 domains, got %d: %v", len(domains), domains)
	}

	expected := []string{"discord.com", "gateway.discord.gg", "cdn.discordapp.com"}
	for i, exp := range expected {
		if domains[i] != exp {
			t.Errorf("Domain mismatch at %d: expected %s, got %s", i, exp, domains[i])
		}
	}
}

func TestLoadCustomConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "custom.toml")

	content := `[general]
backend = "native"
strategy = "strategy_d"
prefer_ipv4 = false

[dns]
mode = "doh"
doh_provider = "quad9"
update_interval_sec = 60
sync_hosts = true
local_dns_port = 5354

[firewall]
driver = "iptables"
queue_num = 300
proxy_port = 12345
block_quic = false
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed writing custom config: %v", err)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.General.Backend != "native" {
		t.Errorf("Expected backend native, got %s", cfg.General.Backend)
	}
	if cfg.General.Strategy != "strategy_d" {
		t.Errorf("Expected strategy strategy_d, got %s", cfg.General.Strategy)
	}
	if cfg.General.PreferIPv4 {
		t.Errorf("Expected PreferIPv4 to be false")
	}
	if !cfg.DNS.SyncHosts {
		t.Errorf("Expected SyncHosts to be true")
	}
	if cfg.DNS.LocalDNSPort != 5354 {
		t.Errorf("Expected LocalDNSPort 5354, got %d", cfg.DNS.LocalDNSPort)
	}
	if cfg.Firewall.BlockQUIC {
		t.Errorf("Expected BlockQUIC to be false")
	}
	if cfg.Firewall.Driver != "iptables" {
		t.Errorf("Expected driver iptables, got %s", cfg.Firewall.Driver)
	}
	if cfg.Firewall.QueueNum != 300 {
		t.Errorf("Expected QueueNum 300, got %d", cfg.Firewall.QueueNum)
	}
	if cfg.Firewall.ProxyPort != 12345 {
		t.Errorf("Expected ProxyPort 12345, got %d", cfg.Firewall.ProxyPort)
	}
}
