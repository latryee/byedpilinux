package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/utils"
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

	if !cfg.DNS.SyncHosts {
		t.Errorf("Expected SyncHosts to be true by default")
	}

	if cfg.DNS.LocalDNSPort != 0 {
		t.Errorf("Expected LocalDNSPort to be 0 by default, got %d", cfg.DNS.LocalDNSPort)
	}
}

func TestTOMLArrayWithCommas(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "array_test.toml")

	content := `[nfqws]
strategy_c_args = ["--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=sniext+2,midsld", "--dpi-desync-fooling=badsum"]
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed writing array test config: %v", err)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	expected := []string{
		"--dpi-desync=fake,multisplit",
		"--dpi-desync-split-pos=sniext+2,midsld",
		"--dpi-desync-fooling=badsum",
	}

	if len(cfg.NFQWS.StrategyCArgs) != len(expected) {
		t.Fatalf("Expected %d arguments, got %d: %v", len(expected), len(cfg.NFQWS.StrategyCArgs), cfg.NFQWS.StrategyCArgs)
	}

	for i, exp := range expected {
		if cfg.NFQWS.StrategyCArgs[i] != exp {
			t.Errorf("Arg %d mismatch: expected %q, got %q", i, exp, cfg.NFQWS.StrategyCArgs[i])
		}
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

func TestRuntimeStatusSerialization(t *testing.T) {
	st := &utils.RuntimeStatus{
		PID:            1234,
		StartTime:      time.Now().Add(-10 * time.Minute),
		UpdateTime:     time.Now(),
		ServiceActive:  true,
		BackendName:    "nfqws (Netfilter queue)",
		BackendRunning: true,
		BackendPID:     1235,
		ActiveStrategy: "strategy_c",
		StrategyName:   "TLS SNI Fake Split (fakedsplit midsld ttl=6)",
		FirewallDriver: "nftables",
		FirewallActive: true,
		Packets:        42,
		Bytes:          1024,
		DNSMode:        "doh",
		DoHProvider:    "cloudflare",
		SyncHosts:      true,
		TargetDomains:  16,
	}

	tmpFile := filepath.Join(t.TempDir(), "status.json")
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	readData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var readSt utils.RuntimeStatus
	if err := json.Unmarshal(readData, &readSt); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if readSt.Packets != 42 || !readSt.FirewallActive || readSt.ActiveStrategy != "strategy_c" {
		t.Errorf("Runtime status fields mismatch: %+v", readSt)
	}
}

func TestUpdateStrategyInConfigAndRollback(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.toml")

	initialContent := `# Configuration test file
[general]
log_level = "info"
backend = "nfqws"
strategy = "strategy_c"

[dns]
mode = "doh"
sync_hosts = true

[nfqws]
strategy_c_args = ["--dpi-desync=fakedsplit", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=6"]
`
	if err := os.WriteFile(configFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed writing test config: %v", err)
	}

	// 1. Update strategy to strategy_d
	if err := config.UpdateStrategyInConfig(configFile, "strategy_d", nil); err != nil {
		t.Fatalf("UpdateStrategyInConfig failed: %v", err)
	}

	// 2. Verify updated config
	updatedCfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed on updated file: %v", err)
	}
	if updatedCfg.General.Strategy != "strategy_d" {
		t.Errorf("Expected updated strategy strategy_d, got %s", updatedCfg.General.Strategy)
	}
	if updatedCfg.General.Backend != "nfqws" || !updatedCfg.DNS.SyncHosts {
		t.Errorf("Unrelated config settings altered unexpectedly: %+v", updatedCfg)
	}

	// 3. Verify backup file exists and has previous strategy
	bakFile := configFile + ".autobak"
	if _, err := os.Stat(bakFile); err != nil {
		t.Fatalf("Expected backup file %s to exist: %v", bakFile, err)
	}
	bakCfg, err := config.LoadConfig(bakFile)
	if err != nil {
		t.Fatalf("LoadConfig failed on backup file: %v", err)
	}
	if bakCfg.General.Strategy != "strategy_c" {
		t.Errorf("Expected backup strategy strategy_c, got %s", bakCfg.General.Strategy)
	}

	// 4. Test Rollback
	if err := config.RestoreConfigBackup(configFile); err != nil {
		t.Fatalf("RestoreConfigBackup failed: %v", err)
	}
	restoredCfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed on restored file: %v", err)
	}
	if restoredCfg.General.Strategy != "strategy_c" {
		t.Errorf("Expected restored strategy strategy_c, got %s", restoredCfg.General.Strategy)
	}
}

