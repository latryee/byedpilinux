package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	General     GeneralConfig     `json:"general"`
	DNS         DNSConfig         `json:"dns"`
	Firewall    FirewallConfig    `json:"firewall"`
	NFQWS       NFQWSConfig       `json:"nfqws"`
	Native      NativeConfig      `json:"native"`
	Diagnostics DiagnosticsConfig `json:"diagnostics"`
	Domains     []string          `json:"domains"`
	ConfigPath  string            `json:"config_path"`
}

type GeneralConfig struct {
	LogLevel    string `json:"log_level"`
	DomainsFile string `json:"domains_file"`
	PreferIPv4  bool   `json:"prefer_ipv4"`
	Backend     string `json:"backend"`
	Strategy    string `json:"strategy"`
}

type DNSConfig struct {
	Mode              string `json:"mode"`
	DoHProvider       string `json:"doh_provider"`
	CustomResolver    string `json:"custom_resolver"`
	UpdateIntervalSec int    `json:"update_interval_sec"`
	SyncHosts         bool   `json:"sync_hosts"`
	LocalDNSPort      int    `json:"local_dns_port"`
}

type FirewallConfig struct {
	Driver     string `json:"driver"`
	TableName  string `json:"table_name"`
	QueueNum   int    `json:"queue_num"`
	ProxyPort  int    `json:"proxy_port"`
	ManageIPv6 bool   `json:"manage_ipv6"`
	BlockQUIC  bool   `json:"block_quic"`
}

type NFQWSConfig struct {
	BinaryPath    string   `json:"binary_path"`
	StrategyCArgs []string `json:"strategy_c_args"`
	StrategyDArgs []string `json:"strategy_d_args"`
}

type NativeConfig struct {
	SplitPos     int `json:"split_pos"`
	SplitDelayMs int `json:"split_delay_ms"`
}

type DiagnosticsConfig struct {
	TimeoutSec      int    `json:"timeout_sec"`
	APIEndpoint     string `json:"api_endpoint"`
	GatewayEndpoint string `json:"gateway_endpoint"`
	CDNEndpoint     string `json:"cdn_endpoint"`
	VoiceEndpoint   string `json:"voice_endpoint"`
}

func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			LogLevel:    "info",
			DomainsFile: "/etc/discord-bypass/domains.txt",
			PreferIPv4:  true,
			Backend:     "nfqws",
			Strategy:    "strategy_c",
		},
		DNS: DNSConfig{
			Mode:              "doh",
			DoHProvider:       "cloudflare",
			CustomResolver:    "1.1.1.1:53",
			UpdateIntervalSec: 300,
			SyncHosts:         true,
			LocalDNSPort:      5354,
		},
		Firewall: FirewallConfig{
			Driver:     "auto",
			TableName:  "discord_bypass",
			QueueNum:   200,
			ProxyPort:  10443,
			ManageIPv6: true,
			BlockQUIC:  false,
		},
		NFQWS: NFQWSConfig{
			BinaryPath:    "/usr/local/bin/discord-bypass-nfqws",
			StrategyCArgs: []string{"--dpi-desync=split2"},
			StrategyDArgs: []string{"--dpi-desync=fake,split2", "--dpi-desync-ttl=4"},
		},
		Native: NativeConfig{
			SplitPos:     2,
			SplitDelayMs: 2,
		},
		Diagnostics: DiagnosticsConfig{
			TimeoutSec:      5,
			APIEndpoint:     "https://discord.com/api/v9/gateway",
			GatewayEndpoint: "wss://gateway.discord.gg",
			CDNEndpoint:     "https://cdn.discordapp.com",
			VoiceEndpoint:   "voice.discord.media:443",
		},
		Domains: []string{
			"discord.com",
			"gateway.discord.gg",
			"cdn.discordapp.com",
			"media.discordapp.net",
			"router.discordapp.net",
			"discord.gg",
			"discordapp.com",
			"discordapp.net",
			"discordstatus.com",
			"latency.discord.media",
		},
	}
}

// LoadConfig loads configuration from the given file or fallback search paths.
func LoadConfig(specifiedPath string) (*Config, error) {
	cfg := DefaultConfig()

	var targetPath string
	if specifiedPath != "" {
		if _, err := os.Stat(specifiedPath); err == nil {
			targetPath = specifiedPath
		} else {
			return nil, fmt.Errorf("specified config file not found: %s", specifiedPath)
		}
	} else {
		candidates := []string{
			"/etc/discord-bypass/config.toml",
			"./config/config.toml",
			"./config.toml",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				targetPath = c
				break
			}
		}
	}

	if targetPath == "" {
		// Use defaults if no config file found
		return cfg, nil
	}

	cfg.ConfigPath = targetPath
	if err := parseTOML(targetPath, cfg); err != nil {
		return nil, fmt.Errorf("error parsing config %s: %w", targetPath, err)
	}

	// Load domains
	domainsFile := cfg.General.DomainsFile
	if _, err := os.Stat(domainsFile); os.IsNotExist(err) {
		// Try relative to config dir
		altPath := filepath.Join(filepath.Dir(targetPath), "domains.txt")
		if _, err2 := os.Stat(altPath); err2 == nil {
			domainsFile = altPath
		}
	}

	if loadedDomains, err := LoadDomains(domainsFile); err == nil && len(loadedDomains) > 0 {
		cfg.Domains = loadedDomains
	}

	return cfg, nil
}

// LoadDomains reads domains from a flat file, ignoring empty lines and comments
func LoadDomains(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip comments on same line
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		lower := strings.ToLower(line)
		if !seen[lower] {
			seen[lower] = true
			domains = append(domains, lower)
		}
	}
	return domains, scanner.Err()
}

// Lightweight standard-library TOML parser for sections and key=value pairs
func parseTOML(path string, cfg *Config) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	currentSection := ""
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle section headers [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.Trim(line, "[] \t"))
			continue
		}

		// Handle key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		// Strip inline comment if not quoted
		if !strings.HasPrefix(val, "\"") && !strings.HasPrefix(val, "[") {
			if idx := strings.Index(val, "#"); idx != -1 {
				val = strings.TrimSpace(val[:idx])
			}
		}

		assignConfigValue(cfg, currentSection, key, val)
	}

	return scanner.Err()
}

func assignConfigValue(cfg *Config, section, key, rawVal string) {
	// Parse strings, booleans, ints, string arrays
	cleanStr := strings.Trim(rawVal, "\"")
	boolVal, _ := strconv.ParseBool(cleanStr)
	intVal, _ := strconv.Atoi(cleanStr)

	parseStringArray := func(v string) []string {
		v = strings.Trim(v, "[]")
		items := strings.Split(v, ",")
		var res []string
		for _, it := range items {
			trimmed := strings.Trim(strings.TrimSpace(it), "\"")
			if trimmed != "" {
				res = append(res, trimmed)
			}
		}
		return res
	}

	switch section {
	case "general":
		switch key {
		case "log_level":
			cfg.General.LogLevel = cleanStr
		case "domains_file":
			cfg.General.DomainsFile = cleanStr
		case "prefer_ipv4":
			cfg.General.PreferIPv4 = boolVal
		case "backend":
			cfg.General.Backend = cleanStr
		case "strategy":
			cfg.General.Strategy = cleanStr
		}
	case "dns":
		switch key {
		case "mode":
			cfg.DNS.Mode = cleanStr
		case "doh_provider":
			cfg.DNS.DoHProvider = cleanStr
		case "custom_resolver":
			cfg.DNS.CustomResolver = cleanStr
		case "update_interval_sec":
			if intVal > 0 {
				cfg.DNS.UpdateIntervalSec = intVal
			}
		case "sync_hosts":
			cfg.DNS.SyncHosts = boolVal
		case "local_dns_port":
			if intVal > 0 {
				cfg.DNS.LocalDNSPort = intVal
			}
		}
	case "firewall":
		switch key {
		case "driver":
			cfg.Firewall.Driver = cleanStr
		case "table_name":
			cfg.Firewall.TableName = cleanStr
		case "queue_num":
			if intVal > 0 {
				cfg.Firewall.QueueNum = intVal
			}
		case "proxy_port":
			if intVal > 0 {
				cfg.Firewall.ProxyPort = intVal
			}
		case "manage_ipv6":
			cfg.Firewall.ManageIPv6 = boolVal
		case "block_quic":
			cfg.Firewall.BlockQUIC = boolVal
		}
	case "nfqws":
		switch key {
		case "binary_path":
			cfg.NFQWS.BinaryPath = cleanStr
		case "strategy_c_args":
			cfg.NFQWS.StrategyCArgs = parseStringArray(rawVal)
		case "strategy_d_args":
			cfg.NFQWS.StrategyDArgs = parseStringArray(rawVal)
		}
	case "native":
		switch key {
		case "split_pos":
			if intVal > 0 {
				cfg.Native.SplitPos = intVal
			}
		case "split_delay_ms":
			if intVal >= 0 {
				cfg.Native.SplitDelayMs = intVal
			}
		}
	case "diagnostics":
		switch key {
		case "timeout_sec":
			if intVal > 0 {
				cfg.Diagnostics.TimeoutSec = intVal
			}
		case "api_endpoint":
			cfg.Diagnostics.APIEndpoint = cleanStr
		case "gateway_endpoint":
			cfg.Diagnostics.GatewayEndpoint = cleanStr
		case "cdn_endpoint":
			cfg.Diagnostics.CDNEndpoint = cleanStr
		case "voice_endpoint":
			cfg.Diagnostics.VoiceEndpoint = cleanStr
		}
	}
}
