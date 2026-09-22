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
	fmt.Println("Usage: discord-bypass <subcommand> [options]\n")
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
	fmt.Println("  test [--discord]   Verify end-to-end connectivity (DNS, TLS, REST API, firewall counters)")
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
	if isSystemdRunning() {
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

	var pkts, bytes uint64
	fwActive := fw.IsActive()
	if fwActive {
		pkts, bytes, _ = fw.GetStats()
		fmt.Printf("Firewall Rules     : \033[32mACTIVE\033[0m (Traffic: %d pkts, %d bytes)\n", pkts, bytes)
	} else {
		fmt.Printf("Firewall Rules     : \033[33mINACTIVE\033[0m\n")
	}

	if serviceActive && fwActive && pkts > 0 {
		fmt.Printf("Bypass Health      : \033[32mOPERATIONAL (Traffic intercepted and processed)\033[0m\n")
	} else if serviceActive {
		fmt.Printf("Bypass Health      : \033[33mRUNNING (Unverified - 0 packets intercepted yet)\033[0m\n")
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
	if tlsOk && apiOk {
		fmt.Println("\033[32mVERDICT: Discord bypass is fully verified and functional!\033[0m")
	} else {
		fmt.Println("\033[31mVERDICT: Discord bypass is NOT fully functional.\033[0m")
		fmt.Println("Run 'discord-bypass diagnose' for complete details and remediation.")
	}
	fmt.Println("============================================================")
}

func cmdDiagnose() {
	fmt.Println(Banner)
	fmt.Println("Running comprehensive 12-point network diagnostic...\n")

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

	// 1. Stop systemd service if running
	_ = exec.Command("systemctl", "stop", "discord-bypass").Run()

	// 2. Kill any stray daemon processes
	_ = exec.Command("pkill", "-9", "-f", "discord-bypass daemon").Run()
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

[Service]
Type=simple
ExecStart=/usr/local/bin/discord-bypass daemon
ExecStop=/usr/local/bin/discord-bypass emergency-disable
Restart=on-failure
RestartSec=3s
StartLimitIntervalSec=60s
StartLimitBurst=5
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
LimitNOFILE=65535

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

	// 4. Start local DNS proxy and configure systemd-resolved split-DNS if configured
	var dnsProxy *dns.DNSProxy
	if cfg.DNS.LocalDNSPort > 0 {
		listenAddr := fmt.Sprintf("127.0.0.1:%d", cfg.DNS.LocalDNSPort)
		dnsProxy = dns.NewDNSProxy(listenAddr, cfg.DNS.DoHProvider, cfg.General.PreferIPv4)
		if err := dnsProxy.Start(ctx); err != nil {
			utils.Warn("Failed to start local DNS proxy on %s: %v", listenAddr, err)
		} else {
			utils.Info("Local loopback DNS proxy listening on %s", listenAddr)
			defaultIface := ""
			if sys != nil {
				defaultIface = sys.DefaultInterface
			}
			if err := dns.ConfigureSystemdResolved(defaultIface, listenAddr); err != nil {
				utils.Debug("systemd-resolved routing domain config note: %v", err)
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
