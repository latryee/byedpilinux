package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"discord-bypass/src/pkg/backend"
	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/diagnostics"
	"discord-bypass/src/pkg/dns"
	"discord-bypass/src/pkg/firewall"
	"discord-bypass/src/pkg/strategy"
	"discord-bypass/src/pkg/utils"
)

const (
	Version = "1.0.0"
	Banner  = `
============================================================
      discord-bypass - Linux DPI Circumvention Tool        
               Optimized for Discord in TR                  
============================================================`
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	subcommand := strings.ToLower(os.Args[1])

	switch subcommand {
	case "install":
		cmdInstall()
	case "uninstall":
		cmdUninstall()
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "restart":
		cmdRestart()
	case "status":
		cmdStatus()
	case "test":
		cmdTest()
	case "diagnose":
		cmdDiagnose()
	case "emergency-disable", "restore":
		cmdEmergencyDisable()
	case "logs":
		cmdLogs()
	case "daemon":
		cmdDaemon()
	case "version", "-v", "--version":
		fmt.Printf("discord-bypass version %s\n", Version)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\nRun 'discord-bypass help' for usage.\n", subcommand)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(Banner)
	fmt.Printf("Version: %s\n\n", Version)
	fmt.Println("Usage: discord-bypass <subcommand> [options]")
	fmt.Println()
	fmt.Println("Management Commands (requires sudo):")
	fmt.Println("  install            Install discord-bypass binaries, config, and systemd service")
	fmt.Println("  uninstall          Completely purge discord-bypass, systemd service, and firewall rules")
	fmt.Println("  start              Start the bypass service and activate DPI circumvention")
	fmt.Println("  stop               Stop the bypass service and cleanly deactivate firewall rules")
	fmt.Println("  restart            Restart the bypass service")
	fmt.Println("  emergency-disable  Immediately purge all firewall rules and kill bypass processes")
	fmt.Println("")
	fmt.Println("Inspection & Diagnostic Commands (unprivileged):")
	fmt.Println("  status             Display current service, backend, firewall, and traffic status")
	fmt.Println("  test [--cidrs]     Verify end-to-end connectivity or inspect normalized CIDRs")
	fmt.Println("  diagnose           Run deep 12-point diagnostic (DNS, SNI RST, API, Gateway, Voice, ISP)")
	fmt.Println("  logs               View recent systemd service logs")
	fmt.Println("  help               Show this help message")
}

func requireRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: This command requires root privileges. Please run with sudo:\n  sudo discord-bypass %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdStart() {
	// If systemctl is available and systemd is PID 1, use systemctl
	if isSystemdRunning() {
		requireRoot()
		fmt.Println("Starting discord-bypass service via systemd...")
		cmd := exec.Command("systemctl", "start", "discord-bypass")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start service: %v\nRun 'discord-bypass logs' to inspect errors.\n", err)
			os.Exit(1)
		}
		fmt.Println("[OK] discord-bypass service started successfully.")
		time.Sleep(500 * time.Millisecond)
		cmdStatus()
	} else {
		// Fallback for non-systemd or container: run daemon
		cmdDaemon()
	}
}

func cmdStop() {
	requireRoot()
	if isSystemdRunning() && os.Getenv("INVOCATION_ID") == "" {
		fmt.Println("Stopping discord-bypass service via systemd...")
		_ = exec.Command("systemctl", "stop", "discord-bypass").Run()
	}
	// Revert DNS modifications and purge active rules
	_ = dns.RemoveDiscordHosts()
	_ = dns.RevertSystemdResolved("")
	_ = firewall.EmergencyDisable()
	fmt.Println("[OK] discord-bypass stopped, DNS overrides reverted, and firewall rules deactivated.")
}

func cmdRestart() {
	requireRoot()
	if isSystemdRunning() {
		fmt.Println("Restarting discord-bypass service...")
		_ = exec.Command("systemctl", "restart", "discord-bypass").Run()
		time.Sleep(500 * time.Millisecond)
		cmdStatus()
	} else {
		cmdStop()
		cmdStart()
	}
}

func cmdStatus() {
	cfg, _ := config.LoadConfig("")
	sys, _ := utils.DetectSystem()
	fw, _ := firewall.NewFirewallManager(cfg, sys)

	fmt.Println("============================================================")
	fmt.Println("                  discord-bypass Status                     ")
	fmt.Println("============================================================")

	// Check systemd service
	serviceActive := false
	if out, err := exec.Command("systemctl", "is-active", "discord-bypass").Output(); err == nil {
		statusStr := strings.TrimSpace(string(out))
		if statusStr == "active" {
			serviceActive = true
			fmt.Printf("Service Status     : \033[32mACTIVE (running)\033[0m\n")
		} else {
			fmt.Printf("Service Status     : \033[33mINACTIVE (%s)\033[0m\n", statusStr)
		}
	} else {
		// Check process
		outPs, _ := exec.Command("pgrep", "-f", "discord-bypass daemon").Output()
		if len(strings.TrimSpace(string(outPs))) > 0 {
			serviceActive = true
			fmt.Printf("Service Status     : \033[32mACTIVE (process running)\033[0m\n")
		} else {
			fmt.Printf("Service Status     : \033[33mINACTIVE (stopped)\033[0m\n")
		}
	}

	strat, _ := strategy.GetStrategy(cfg.General.Strategy)
	fmt.Printf("Configured Backend : %s\n", cfg.General.Backend)
	fmt.Printf("Active Strategy    : %s (%s)\n", strat.Name, cfg.General.Strategy)
	fmt.Printf("DNS Mode           : %s (Provider: %s)\n", cfg.DNS.Mode, cfg.DNS.DoHProvider)
	fmt.Printf("Hosts Sync         : %v\n", cfg.DNS.SyncHosts)
	if dns.HasDiscordHosts() {
		fmt.Printf("Hosts Override     : \033[32mACTIVE (/etc/hosts mapped)\033[0m\n")
	} else {
		fmt.Printf("Hosts Override     : \033[33mINACTIVE\033[0m\n")
	}
	fmt.Printf("IPv4 Preference    : %v\n", cfg.General.PreferIPv4)
	fmt.Printf("Firewall Driver    : %s\n", fw.DriverName())

	// Check DNS Poisoning status
	isPoisoned, sysIPs, _, _, _ := dns.DetectPoisoning("discord.com", cfg.DNS.DoHProvider)
	if isPoisoned {
		fmt.Printf("DNS Status         : \033[31mDNS POISONED\033[0m (Resolving to sinkhole %v)\n", sysIPs)
	} else {
		fmt.Printf("DNS Status         : \033[32mDNS FIXED\033[0m (Resolving cleanly to %v)\n", sysIPs)
	}

	var pkts, bytes uint64
	fwActive := fw.IsActive()
	if fwActive {
		pkts, bytes, _ = fw.GetStats()
		if pkts > 0 {
			fmt.Printf("Firewall Rules     : \033[32mFIREWALL ACTIVE (TRAFFIC INTERCEPTED)\033[0m (%d pkts, %d bytes)\n", pkts, bytes)
		} else {
			fmt.Printf("Firewall Rules     : \033[32mFIREWALL ACTIVE\033[0m (0 pkts yet)\n")
		}
	} else {
		fmt.Printf("Firewall Rules     : \033[33mINACTIVE\033[0m\n")
	}

	if serviceActive && fwActive && pkts > 0 && !isPoisoned {
		fmt.Printf("Bypass Health      : \033[32mBYPASS VERIFIED (Traffic intercepted and processed)\033[0m\n")
	} else if serviceActive && !isPoisoned {
		fmt.Printf("Bypass Health      : \033[33mRUNNING (UNVERIFIED - 0 packets intercepted yet)\033[0m\n")
	} else if serviceActive && isPoisoned {
		fmt.Printf("Bypass Health      : \033[31mFAILED (DNS POISONED)\033[0m\n")
	} else {
		fmt.Printf("Bypass Health      : \033[31mSTOPPED\033[0m\n")
	}

	fmt.Printf("Targeted Domains   : %d domains loaded\n", len(cfg.Domains))
	fmt.Println("------------------------------------------------------------")

	// Quick connectivity probe
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d := net.Dialer{Timeout: 2 * time.Second}
	if conn, err := d.DialContext(ctx, "tcp", "discord.com:443"); err == nil {
		remoteAddr := conn.RemoteAddr().String()
		conn.Close()
		fmt.Printf("Discord TCP (443)  : \033[32m[OK] Connected to %s\033[0m\n", remoteAddr)
	} else {
		fmt.Printf("Discord TCP (443)  : \033[31m[FAIL] %v\033[0m\n", err)
	}

	if !serviceActive {
		fmt.Println("\nTip: To activate the bypass, run: 'sudo discord-bypass start'")
	}
	fmt.Println("============================================================")
}

func cmdTest() {
	showCIDRs := false
	for _, a := range os.Args[2:] {
		if a == "--cidrs" || a == "-c" {
			showCIDRs = true
		}
	}

	if showCIDRs {
		fmt.Println("============================================================")
		fmt.Println("       discord-bypass CIDR Normalization Diagnostic         ")
		fmt.Println("============================================================")
		fmt.Printf("Raw Default IPv4 CIDRs (%d elements):\n", len(firewall.DefaultDiscordIPv4CIDRs))
		for _, c := range firewall.DefaultDiscordIPv4CIDRs {
			fmt.Printf("  - %s\n", c)
		}
		normV4, _, err := firewall.NormalizeCIDRs(firewall.DefaultDiscordIPv4CIDRs)
		if err != nil {
			fmt.Printf("\n  \033[31m[FAIL]\033[0m Normalization error: %v\n", err)
			return
		}
		fmt.Printf("\nNormalized Disjoint IPv4 Intervals (%d elements):\n", len(normV4))
		for _, c := range normV4 {
			fmt.Printf("  + %s\n", c)
		}
		fmt.Println("\nSubsumption Analysis:")
		for _, raw := range firewall.DefaultDiscordIPv4CIDRs {
			containedIn := ""
			rawNet, _ := firewall.ParseCIDRorIP(raw)
			for _, norm := range normV4 {
				normNet, _ := firewall.ParseCIDRorIP(norm)
				if raw != norm && firewall.Covers(normNet, rawNet) {
					containedIn = norm
					break
				}
			}
			if containedIn != "" {
				fmt.Printf("  * %-18s -> Subsumed by %s (redundant interval pruned)\n", raw, containedIn)
			} else {
				fmt.Printf("  * %-18s -> Retained as base interval\n", raw)
			}
		}

		_, normV6All, _ := firewall.NormalizeCIDRs(firewall.DefaultDiscordIPv6CIDRs)
		if len(normV6All) > 0 {
			fmt.Printf("\nNormalized IPv6 Intervals (%d elements):\n", len(normV6All))
			for _, c := range normV6All {
				fmt.Printf("  + %s\n", c)
			}
		}

		// Validate with nft -c if nft is available
		if _, err := exec.LookPath("nft"); err == nil {
			testScript := fmt.Sprintf("add table inet test_discord_bypass\nadd set inet test_discord_bypass s { type ipv4_addr; flags interval; elements = { %s }; }\ndelete table inet test_discord_bypass\n", strings.Join(normV4, ", "))
			checkCmd := exec.Command("nft", "-c", "-f", "-")
			checkCmd.Stdin = strings.NewReader(testScript)
			out, chkErr := checkCmd.CombinedOutput()
			if chkErr == nil {
				fmt.Printf("\n\033[32m[OK] nftables syntax validation passed (nft -c)\033[0m\n")
			} else if strings.Contains(string(out), "Operation not permitted") {
				unshareCmd := exec.Command("unshare", "-r", "-n", "nft", "-c", "-f", "-")
				unshareCmd.Stdin = strings.NewReader(testScript)
				if uOut, uErr := unshareCmd.CombinedOutput(); uErr == nil {
					fmt.Printf("\n\033[32m[OK] nftables syntax validation passed via unshare namespace (nft -c)\033[0m\n")
				} else {
					fmt.Printf("\n\033[33m[WARN]\033[0m nft -c check note: %s\n", strings.TrimSpace(string(uOut)))
				}
			} else {
				fmt.Printf("\n\033[31m[FAIL]\033[0m nft -c syntax validation failed: %s\n", strings.TrimSpace(string(out)))
			}
		}
		fmt.Println("============================================================")
		return
	}

	cfg, _ := config.LoadConfig("")
	sys, _ := utils.DetectSystem()
	fw, _ := firewall.NewFirewallManager(cfg, sys)

	fmt.Println("============================================================")
	fmt.Println("          discord-bypass Connectivity Verification          ")
	fmt.Println("============================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Initial Firewall stats
	var initPkts uint64
	if fw != nil && fw.IsActive() {
		initPkts, _, _ = fw.GetStats()
	}

	// 2. DNS Verification
	fmt.Println("\n[1/5] Checking DNS Resolution...")
	isPoisoned, sysIPs, dohIPs, details, err := dns.DetectPoisoning("discord.com", cfg.DNS.DoHProvider)
	if err != nil {
		fmt.Printf("  \033[31m[FAIL]\033[0m DNS lookup failed: %v\n", err)
	} else if isPoisoned {
		fmt.Printf("  \033[31m[FAIL]\033[0m DNS Poisoning/Sinkhole detected!\n         System DNS resolved to: %v\n         Secure DoH resolved to: %v\n         %s\n", sysIPs, dohIPs, details)
	} else {
		fmt.Printf("  \033[32m[OK]\033[0m   DNS resolution clean: %v\n", sysIPs)
	}
	if dns.HasDiscordHosts() {
		fmt.Println("  \033[32m[OK]\033[0m   /etc/hosts override is ACTIVE (routing to clean DoH IPs)")
	} else {
		fmt.Println("  \033[33m[INFO]\033[0m /etc/hosts override is inactive")
	}

	// 3. TCP Handshake tests
	fmt.Println("\n[2/5] Testing TCP Handshakes...")
	endpoints := []struct {
		name string
		addr string
	}{
		{"Discord Web & API", "discord.com:443"},
		{"Discord WebSocket Gateway", "gateway.discord.gg:443"},
		{"Discord CDN Assets", "cdn.discordapp.com:443"},
	}
	for _, ep := range endpoints {
		start := time.Now()
		d := net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", ep.addr)
		lat := time.Since(start)
		if err != nil {
			fmt.Printf("  \033[31m[FAIL]\033[0m %-26s : %v\n", ep.name, err)
		} else {
			rAddr := conn.RemoteAddr().String()
			conn.Close()
			fmt.Printf("  \033[32m[OK]\033[0m   %-26s : %v (%s)\n", ep.name, lat, rAddr)
		}
	}

	// 4. TLS & Certificate Verification
	fmt.Println("\n[3/5] Testing TLS SNI & Certificate Integrity...")
	tlsDialer := &net.Dialer{Timeout: 4 * time.Second}
	tlsConn, err := tls.DialWithDialer(tlsDialer, "tcp", "discord.com:443", &tls.Config{
		ServerName:         "discord.com",
		InsecureSkipVerify: false,
	})
	tlsOk := false
	if err != nil {
		fmt.Printf("  \033[31m[FAIL]\033[0m TLS Handshake failed: %v\n", err)
		// Check if intercepted by sinkhole certificate
		probeConn, probeErr := tls.DialWithDialer(tlsDialer, "tcp", "discord.com:443", &tls.Config{
			ServerName:         "discord.com",
			InsecureSkipVerify: true,
		})
		if probeErr == nil {
			defer probeConn.Close()
			if certs := probeConn.ConnectionState().PeerCertificates; len(certs) > 0 {
				fmt.Printf("         \033[31mIntercepted by: %s (Issuer: %s, SANs: %v)\033[0m\n",
					certs[0].Subject.CommonName, certs[0].Issuer.CommonName, certs[0].DNSNames)
			}
		}
	} else {
		tlsOk = true
		rAddr := tlsConn.RemoteAddr().String()
		state := tlsConn.ConnectionState()
		tlsConn.Close()
		fmt.Printf("  \033[32m[OK]\033[0m   TLS negotiated: %s (%s) with %s\n",
			tlsVersionToString(state.Version), tls.CipherSuiteName(state.CipherSuite), rAddr)
	}

	// 5. High-level REST API probe
	fmt.Println("\n[4/5] Testing Discord Application Layer Endpoints...")
	httpClient := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v9/gateway", nil)
	req.Header.Set("User-Agent", "discord-bypass/1.0")
	resp, err := httpClient.Do(req)
	apiOk := false
	if err != nil {
		fmt.Printf("  \033[31m[FAIL]\033[0m Discord API (/api/v9/gateway): %v\n", err)
	} else {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		if resp.StatusCode == 200 {
			apiOk = true
			fmt.Printf("  \033[32m[OK]\033[0m   Discord API: HTTP 200 OK (%s)\n", strings.TrimSpace(string(body)))
		} else {
			fmt.Printf("  \033[33m[WARN]\033[0m Discord API: HTTP %d (%s)\n", resp.StatusCode, strings.TrimSpace(string(body)))
		}
	}

	// 6. Firewall Counter Verification
	fmt.Println("\n[5/5] Checking Firewall Rule Packet Interception...")
	if fw != nil && fw.IsActive() {
		finalPkts, finalBytes, _ := fw.GetStats()
		delta := finalPkts - initPkts
		if delta > 0 {
			fmt.Printf("  \033[32m[OK]\033[0m   Firewall rule counter incremented: +%d packets (Total: %d pkts, %d bytes)\n",
				delta, finalPkts, finalBytes)
		} else {
			fmt.Printf("  \033[33m[WARN]\033[0m Firewall rules active, but counter did not increment during test (Total: %d pkts)\n",
				finalPkts)
		}
	} else {
		fmt.Println("  \033[33m[INFO]\033[0m Firewall rules not active (Service stopped or unprivileged)")
	}

	fmt.Println("\n============================================================")
	if isPoisoned {
		fmt.Println("\033[31mVERDICT: FAILED (DNS POISONED by ISP sinkhole)\033[0m")
		fmt.Println("System DNS returned sinkhole IP. Start service with: sudo discord-bypass start")
	} else if !tlsOk {
		fmt.Println("\033[31mVERDICT: FAILED (TLS SNI BLOCKED by DPI)\033[0m")
		fmt.Println("TLS handshake blocked or intercepted. Check nfqws and firewall rules.")
	} else if tlsOk && apiOk && fw != nil && fw.IsActive() {
		fmt.Println("\033[32mVERDICT: BYPASS VERIFIED (DNS clean, TLS valid, APIs responsive, firewall active)\033[0m")
	} else if tlsOk && apiOk {
		fmt.Println("\033[33mVERDICT: RUNNING (UNVERIFIED - APIs responsive, check firewall counters)\033[0m")
	} else {
		fmt.Println("\033[31mVERDICT: FAILED (Application layer endpoints unreachable)\033[0m")
	}
	fmt.Println("============================================================")
}

func cmdDiagnose() {
	fmt.Println(Banner)
	fmt.Println("Running comprehensive 12-point network diagnostic...")
	fmt.Println()

	cfg, _ := config.LoadConfig("")
	sys, _ := utils.DetectSystem()
	fw, _ := firewall.NewFirewallManager(cfg, sys)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	report := diagnostics.RunDiagnostics(ctx, cfg, sys, fw)

	for _, item := range report.Items {
		var tag string
		switch item.Status {
		case diagnostics.StatusOk:
			tag = "\033[32m[OK]\033[0m  "
		case diagnostics.StatusFail:
			tag = "\033[31m[FAIL]\033[0m"
		case diagnostics.StatusWarn:
			tag = "\033[33m[WARN]\033[0m"
		default:
			tag = "\033[36m[INFO]\033[0m"
		}

		fmt.Printf("%s %-25s : %s\n", tag, item.Name, item.Details)
	}

	fmt.Println("\n============================================================")
	fmt.Printf("OVERALL VERDICT: %s\n", report.OverallVerdict)
	if report.FailureStage != "None" {
		fmt.Printf("FAILURE STAGE  : %s\n", report.FailureStage)
	}
	if report.Recommendation != "" {
		fmt.Printf("RECOMMENDATION : %s\n", report.Recommendation)
	}
	if report.ISPInfo != nil && report.ISPInfo.Recommendation != "" {
		fmt.Printf("ISP INSIGHT    : %s\n", report.ISPInfo.Recommendation)
	}
	fmt.Println("============================================================")
}

func cmdEmergencyDisable() {
	requireRoot()
	fmt.Println("Executing emergency disable: purging all firewall rules, reverting DNS overrides, and stopping processes...")

	// 1. Stop systemd service if running from interactive CLI
	if isSystemdRunning() && os.Getenv("INVOCATION_ID") == "" {
		_ = exec.Command("systemctl", "stop", "discord-bypass").Run()
	}

	// 2. Kill any stray daemon processes
	if os.Getenv("INVOCATION_ID") == "" {
		_ = exec.Command("pkill", "-9", "-f", "discord-bypass daemon").Run()
	}
	_ = exec.Command("pkill", "-9", "-f", "discord-bypass-nfqws").Run()
	_ = exec.Command("pkill", "-9", "-f", "ciadpi").Run()

	// 3. Revert hosts and systemd-resolved
	_ = dns.RemoveDiscordHosts()
	_ = dns.RevertSystemdResolved("")

	// 4. Purge firewall rules
	err := firewall.EmergencyDisable()
	if err != nil {
		fmt.Printf("[WARN] Firewall purge notes: %v\n", err)
	} else {
		fmt.Println("[OK] All nftables and iptables bypass rules purged cleanly.")
	}

	fmt.Println("[OK] System restored to default unhindered networking state.")
}

func cmdLogs() {
	cmd := exec.Command("journalctl", "-u", "discord-bypass", "-n", "50", "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func cmdInstall() {
	requireRoot()
	fmt.Println(Banner)
	fmt.Println("Starting discord-bypass installation...")

	// 1. Detect system
	sys, err := utils.DetectSystem()
	if err != nil {
		fmt.Fprintf(os.Stderr, "System detection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[INFO] Detected OS: %s (Kernel %s)\n", sys.OSName, sys.KernelVersion)

	// 2. Create directories
	_ = os.MkdirAll("/etc/discord-bypass", 0755)
	_ = os.MkdirAll("/usr/local/bin", 0755)

	// 3. Copy or write default configuration files if not present
	if _, err := os.Stat("/etc/discord-bypass/config.toml"); os.IsNotExist(err) {
		if data, err := os.ReadFile("config/config.toml"); err == nil {
			_ = os.WriteFile("/etc/discord-bypass/config.toml", data, 0644)
			fmt.Println("[OK] Installed /etc/discord-bypass/config.toml")
		}
	} else {
		fmt.Println("[INFO] Existing /etc/discord-bypass/config.toml preserved")
	}

	if _, err := os.Stat("/etc/discord-bypass/domains.txt"); os.IsNotExist(err) {
		if data, err := os.ReadFile("config/domains.txt"); err == nil {
			_ = os.WriteFile("/etc/discord-bypass/domains.txt", data, 0644)
			fmt.Println("[OK] Installed /etc/discord-bypass/domains.txt")
		}
	} else {
		fmt.Println("[INFO] Existing /etc/discord-bypass/domains.txt preserved")
	}

	// 4. Install binary to /usr/local/bin/discord-bypass
	selfPath, err := os.Executable()
	if err == nil {
		targetBin := "/usr/local/bin/discord-bypass"
		if selfPath != targetBin {
			if data, err := os.ReadFile(selfPath); err == nil {
				_ = os.WriteFile(targetBin, data, 0755)
				fmt.Printf("[OK] Installed binary to %s\n", targetBin)
			}
		}
	}

	// 5. Install systemd service unit
	serviceData := `[Unit]
Description=Discord DPI Circumvention Service (discord-bypass)
Documentation=https://github.com/byedpilinux
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=3

[Service]
Type=simple
ExecStart=/usr/local/bin/discord-bypass daemon
ExecStop=/usr/local/bin/discord-bypass emergency-disable
Restart=on-failure
RestartSec=3s
TimeoutStopSec=10s
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
ProtectSystem=full
ProtectHome=read-only
PrivateTmp=true
ReadWritePaths=/etc/discord-bypass /etc/hosts
LimitNOFILE=65535

# Structured Journal Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=discord-bypass

[Install]
WantedBy=multi-user.target
`
	if isSystemdRunning() {
		_ = os.WriteFile("/etc/systemd/system/discord-bypass.service", []byte(serviceData), 0644)
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "enable", "discord-bypass").Run()
		fmt.Println("[OK] Installed and enabled systemd service 'discord-bypass'")
	}

	fmt.Println("\nInstallation Complete!")
	fmt.Println("To start bypass: sudo discord-bypass start")
	fmt.Println("To view status : discord-bypass status")
	fmt.Println("To diagnose    : discord-bypass diagnose")
}

func cmdUninstall() {
	requireRoot()
	fmt.Println("Uninstalling discord-bypass...")

	// 1. Emergency disable & stop
	_ = dns.RemoveDiscordHosts()
	_ = dns.RevertSystemdResolved("")
	_ = firewall.EmergencyDisable()
	if isSystemdRunning() {
		_ = exec.Command("systemctl", "stop", "discord-bypass").Run()
		_ = exec.Command("systemctl", "disable", "discord-bypass").Run()
		_ = os.Remove("/etc/systemd/system/discord-bypass.service")
		_ = exec.Command("systemctl", "daemon-reload").Run()
	}

	// 2. Remove binaries
	_ = os.Remove("/usr/local/bin/discord-bypass")
	_ = os.Remove("/usr/local/bin/discord-bypass-nfqws")

	fmt.Println("[OK] Binaries, systemd service, and DNS overrides cleanly removed.")
	fmt.Println("Configuration directory preserved at /etc/discord-bypass (remove manually if desired: sudo rm -rf /etc/discord-bypass)")
}

func cmdDaemon() {
	requireRoot()
	cfg, err := config.LoadConfig("")
	if err != nil {
		utils.Error("Failed to load configuration: %v", err)
		os.Exit(1)
	}
	utils.GetLogger().SetLevel(cfg.General.LogLevel)
	utils.Info("Starting discord-bypass daemon (v%s)...", Version)

	sys, err := utils.DetectSystem()
	if err != nil {
		utils.Error("System detection error: %v", err)
	}

	strat, err := strategy.GetStrategy(cfg.General.Strategy)
	if err != nil {
		utils.Error("Invalid strategy: %v", err)
		os.Exit(1)
	}
	utils.Info("Selected strategy: %s (%s)", strat.Name, strat.Description)

	fw, err := firewall.NewFirewallManager(cfg, sys)
	if err != nil {
		utils.Error("Failed to initialize firewall driver: %v", err)
		os.Exit(1)
	}

	// Determine firewall mode based on backend
	fwMode := "nfqueue"
	if cfg.General.Backend == "native" {
		fwMode = "redirect"
	}

	// 1. Start packet engine backend if strategy requires it
	var be backend.Backend
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if strat.RequiresBackend {
		be, err = backend.NewBackend(cfg, sys)
		if err != nil {
			utils.Error("Backend initialization failed: %v", err)
			os.Exit(1)
		}

		if err := be.Start(ctx); err != nil {
			utils.Error("Failed to start backend %s: %v", be.Name(), err)
			os.Exit(1)
		}
		utils.Info("DPI circumvention backend %s started", be.Name())
	}

	// 2. Setup Firewall Rules
	if strat.RequiresBackend {
		if err := fw.Setup(fwMode, cfg.Firewall.QueueNum, cfg.Firewall.ProxyPort, cfg.Firewall.BlockQUIC); err != nil {
			utils.Error("Firewall setup failed: %v", err)
			if be != nil {
				_ = be.Stop()
			}
			os.Exit(1)
		}
	}

	// 3. Start DoH Dynamic IP Updater with /etc/hosts sync
	var ipUpdater *dns.IPUpdater
	if cfg.DNS.Mode == "doh" || strat.RequiresDoH {
		ipUpdater = dns.NewIPUpdater(cfg.Domains, cfg.DNS.DoHProvider, cfg.DNS.UpdateIntervalSec, fw, cfg.DNS.SyncHosts)
		ipUpdater.Start()
		utils.Info("Dynamic DoH resolver and IP set updater started (%s, sync_hosts=%v)", cfg.DNS.DoHProvider, cfg.DNS.SyncHosts)
	}

	// 4. Start local domain-routing DNS proxy and configure systemd-resolved
	var dnsProxy *dns.DNSProxy
	if cfg.DNS.LocalDNSPort > 0 {
		listenAddr := fmt.Sprintf("127.0.0.1:%d", cfg.DNS.LocalDNSPort)
		defaultIface := ""
		if sys != nil {
			defaultIface = sys.DefaultInterface
		}
		upstreamDNS := cfg.DNS.UpstreamDNS
		if upstreamDNS == "" && defaultIface != "" {
			upstreamDNS = dns.GetInterfaceDNS(defaultIface)
		}

		dnsProxy = dns.NewDNSProxy(listenAddr, cfg.DNS.DoHProvider, cfg.General.PreferIPv4, upstreamDNS, cfg.Domains)
		if err := dnsProxy.Start(ctx); err != nil {
			utils.Warn("Failed to start local DNS proxy on %s: %v", listenAddr, err)
		} else {
			utils.Info("Local loopback DNS proxy listening on %s (upstream: %s)", listenAddr, dnsProxy.UpstreamDNS())
			if err := dns.ConfigureSystemdResolved(defaultIface, listenAddr); err != nil {
				utils.Debug("systemd-resolved configuration note: %v", err)
			}
		}
	}

	utils.Info("discord-bypass daemon is fully operational and protecting Discord connections")

	// Wait for termination signals (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	utils.Info("Termination signal received. Gracefully shutting down...")

	if dnsProxy != nil {
		dnsProxy.Stop()
		defaultIface := ""
		if sys != nil {
			defaultIface = sys.DefaultInterface
		}
		_ = dns.RevertSystemdResolved(defaultIface)
	}
	if cfg.DNS.SyncHosts {
		_ = dns.RemoveDiscordHosts()
	}
	if ipUpdater != nil {
		ipUpdater.Stop()
	}
	if fw != nil {
		_ = fw.Teardown()
	}
	if be != nil {
		_ = be.Stop()
	}

	utils.Info("Clean shutdown complete.")
}

func tlsVersionToString(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

func isSystemdRunning() bool {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	return false
}
