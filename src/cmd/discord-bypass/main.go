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
	case "notify-status", "notification":
		cmdNotifyStatus()
	case "watch":
		cmdWatch()
	case "tune":
		cmdTune()
	case "strategy":
		cmdStrategy(os.Args[2:])
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
	fmt.Println("Strategy Management & Auto-Tuning:")
	fmt.Println("  tune [--verbose]   Automatically find, verify, and apply a working strategy for current network (requires sudo)")
	fmt.Println("  strategy list      Display all available DPI circumvention strategies and candidates")
	fmt.Println("  strategy current   Display currently active strategy and verification status")
	fmt.Println("  strategy set <id>  Safely switch to a strategy with automatic connectivity verification (requires sudo)")
	fmt.Println("  watch              Watch network state; automatically tune if roaming or interface changes")
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
	fmt.Println("  notify-status      Send desktop notification with service and strategy status")
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
	if prof, err := strategy.LoadNetworkProfile(); err == nil && prof.StrategyID == cfg.General.Strategy && prof.Status == "verified" {
		fmt.Printf("Strategy Status    : \033[32mVERIFIED\033[0m (via %s on %s)\n", prof.Source, prof.VerifiedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Strategy Status    : \033[33mUNVERIFIED\033[0m (run 'sudo discord-bypass tune' to verify)\n")
	}
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
	} else if rtStat, err := utils.ReadRuntimeStatus(); err == nil && rtStat.FirewallActive {
		fwActive = true
		pkts = rtStat.Packets
		bytes = rtStat.Bytes
	}

	if fwActive {
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

	dUDP := net.Dialer{Timeout: 2 * time.Second}
	if connUDP, err := dUDP.DialContext(ctx, "udp", "latency.discord.media:443"); err == nil {
		_, _ = connUDP.Write([]byte{0x00, 0x01, 0x00, 0x00})
		connUDP.Close()
		fmt.Printf("Discord Voice (UDP): \033[32m[OK] WebRTC socket reachable\033[0m\n")
	} else {
		fmt.Printf("Discord Voice (UDP): \033[33m[WARN] %v\033[0m\n", err)
	}

	if connRoblox, err := d.DialContext(ctx, "tcp", "roblox.com:443"); err == nil {
		robloxAddr := connRoblox.RemoteAddr().String()
		connRoblox.Close()
		fmt.Printf("Roblox TCP (443)   : \033[32m[OK] Connected to %s\033[0m\n", robloxAddr)
	} else {
		fmt.Printf("Roblox TCP (443)   : \033[33m[WARN] %v\033[0m\n", err)
	}

	if !serviceActive {
		fmt.Println("\nTip: To activate the bypass, run: 'sudo discord-bypass start'")
	}
	fmt.Println("============================================================")
}

func cmdNotifyStatus() {
	cfg, _ := config.LoadConfig("")
	prof, _ := strategy.LoadNetworkProfile()
	rt, _ := utils.ReadRuntimeStatus()

	stratName := "Unknown"
	stratID := "none"
	if cfg != nil {
		stratID = cfg.General.Strategy
		if s, err := strategy.GetStrategy(stratID); err == nil {
			stratName = s.Name
		}
	}

	serviceActive := false
	if out, err := exec.Command("systemctl", "is-active", "discord-bypass").Output(); err == nil {
		serviceActive = (strings.TrimSpace(string(out)) == "active")
	} else if rt != nil && rt.ServiceActive {
		serviceActive = true
	}

	isVerified := (prof != nil && prof.StrategyID == stratID && prof.Status == "verified")
	var pkts uint64
	if rt != nil {
		pkts = rt.Packets
	}

	var title, body, icon string
	if serviceActive && isVerified {
		title = "Discord Bypass: Active & Verified"
		body = fmt.Sprintf("Strategy: %s\nTraffic: %d packets intercepted and processed", stratName, pkts)
		if prof != nil && prof.NetworkName != "" {
			body += fmt.Sprintf("\nNetwork: %s", prof.NetworkName)
		}
		icon = "discord-bypass"
	} else if serviceActive {
		title = "Discord Bypass: Active (Unverified)"
		body = fmt.Sprintf("Strategy: %s\nRun 'sudo discord-bypass tune' to verify connectivity.", stratName)
		icon = "dialog-warning"
	} else {
		title = "Discord Bypass: Inactive"
		body = "Bypass service is stopped.\nStart with: sudo discord-bypass start"
		icon = "dialog-error"
	}

	fmt.Printf("%s\n%s\n", title, body)

	if notifyPath, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(notifyPath, "-a", "Discord Bypass", "-i", icon, title, body).Run()
	}
}

func cmdWatch() {
	fmt.Println("============================================================")
	fmt.Println("             discord-bypass Network Watcher                 ")
	fmt.Println("============================================================")
	fmt.Println("Monitoring active network interface and connection health...")
	fmt.Println("Press Ctrl+C to stop.")

	lastIface := ""
	if sys, err := utils.DetectSystem(); err == nil && sys != nil {
		lastIface = sys.DefaultInterface
	}
	fmt.Printf("[INFO] Initial interface: %s\n", lastIface)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopping network watcher.")
			return
		case <-ticker.C:
			sys, err := utils.DetectSystem()
			if err != nil || sys == nil {
				continue
			}
			currIface := sys.DefaultInterface
			if currIface != "" && currIface != lastIface {
				fmt.Printf("[!] Network interface change detected: %s -> %s\n", lastIface, currIface)
				lastIface = currIface

				// Test connectivity on new interface
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				res, _ := strategy.VerifyDiscordConnectivity(ctx, 4*time.Second)
				cancel()

				if res == nil || !res.Success {
					fmt.Printf("[WARN] Discord connectivity lost on %s. Triggering notification...\n", currIface)
					if notifyPath, err := exec.LookPath("notify-send"); err == nil {
						_ = exec.Command(notifyPath, "-a", "Discord Bypass", "-i", "dialog-warning",
							"Discord Bağlantısı Kesildi",
							fmt.Sprintf("Ağ arayüzü değişti (%s). 'sudo discord-bypass tune' çalıştırınız.", currIface)).Run()
					}

					// If running with root privileges, auto-tune immediately
					if os.Geteuid() == 0 {
						fmt.Println("[INFO] Auto-tuning bypass for new network...")
						t := strategy.NewTuner("", false)
						if tuneRes, err := t.Run(context.Background()); err == nil && tuneRes != nil && tuneRes.Success {
							if notifyPath, err := exec.LookPath("notify-send"); err == nil {
								_ = exec.Command(notifyPath, "-a", "Discord Bypass", "-i", "discord-bypass",
									"Discord Stratejisi Güncellendi",
									fmt.Sprintf("Yeni ağ (%s) için %s başarıyla ayarlandı.", currIface, tuneRes.SelectedStrategy.Name)).Run()
							}
						}
					}
				} else {
					fmt.Printf("[OK] Discord connectivity verified on %s (latency: %v)\n", currIface, res.Latency)
				}
			}
		}
	}
}

func cmdTune() {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: Automatic strategy tuning requires administrator privileges.\nPlease run:\n  sudo discord-bypass tune\n")
		os.Exit(1)
	}

	verbose := false
	for _, a := range os.Args[2:] {
		if a == "--verbose" || a == "-v" {
			verbose = true
		}
	}

	t := strategy.NewTuner("", verbose)
	_, err := t.Run(context.Background())
	if err != nil {
		os.Exit(1)
	}
}

func cmdStrategy(args []string) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printStrategyHelp()
		return
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "list", "ls":
		cmdStrategyList()
	case "current", "show", "active":
		cmdStrategyCurrent()
	case "set", "use":
		cmdStrategySet(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown strategy subcommand: %q\nRun 'discord-bypass strategy help' for usage.\n", sub)
		os.Exit(1)
	}
}

func printStrategyHelp() {
	fmt.Println("Usage: discord-bypass strategy <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  list               Display all available DPI circumvention strategies and candidates")
	fmt.Println("  current            Display the active strategy and network verification status")
	fmt.Println("  set <strategy-id>  Safely set and verify a strategy (requires sudo)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  discord-bypass strategy list")
	fmt.Println("  discord-bypass strategy current")
	fmt.Println("  sudo discord-bypass strategy set strategy_c")
	fmt.Println("  sudo discord-bypass strategy set strategy_c --force")
}

func cmdStrategyList() {
	cfg, _ := config.LoadConfig("")
	prof, _ := strategy.LoadNetworkProfile()
	strategies := strategy.ListStrategies()

	fmt.Println("==========================================================================================================")
	fmt.Println("                                   Available Discord Bypass Strategies                                    ")
	fmt.Println("==========================================================================================================")
	printfFormat := "%-24s | %-54s | %-9s | %-10s\n"
	fmt.Printf(printfFormat, "Strategy ID", "Name & Parameters", "Status", "Verified")
	fmt.Println("-------------------------|--------------------------------------------------------|-----------|-----------")

	for _, s := range strategies {
		activeTag := ""
		if cfg != nil && s.ID == cfg.General.Strategy {
			activeTag = "[ACTIVE]"
		}

		verifiedTag := ""
		if prof != nil && prof.StrategyID == s.ID && prof.Status == "verified" {
			verifiedTag = "[VERIFIED]"
		}

		fmt.Printf(printfFormat, s.ID, s.Name, activeTag, verifiedTag)
	}

	fmt.Println("==========================================================================================================")
	fmt.Println("Commands:")
	fmt.Println("  * Switch strategy : sudo discord-bypass strategy set <strategy-id>")
	fmt.Println("  * Auto-tune network: sudo discord-bypass tune")
}

func cmdStrategyCurrent() {
	cfgPath := config.GetActiveConfigPath("")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	strat, err := strategy.GetStrategy(cfg.General.Strategy)
	stratName := cfg.General.Strategy
	stratDesc := ""
	var stratArgs []string
	if err == nil {
		stratName = strat.Name
		stratDesc = strat.Description
		stratArgs = strat.NFQWSArgs
	}

	prof, profErr := strategy.LoadNetworkProfile()

	fmt.Println("============================================================")
	fmt.Println("                 Current Network Strategy                   ")
	fmt.Println("============================================================")
	fmt.Printf("Active Strategy ID   : %s\n", cfg.General.Strategy)
	fmt.Printf("Strategy Name        : %s\n", stratName)
	if stratDesc != "" {
		fmt.Printf("Description          : %s\n", stratDesc)
	}
	fmt.Printf("Backend Engine       : %s\n", cfg.General.Backend)
	if len(stratArgs) > 0 {
		fmt.Printf("NFQWS Parameters     : %s\n", strings.Join(stratArgs, " "))
	}
	fmt.Printf("Configuration File   : %s\n", cfgPath)

	if profErr == nil && prof != nil && prof.StrategyID == cfg.General.Strategy && prof.Status == "verified" {
		fmt.Printf("Verification Status  : \033[32mVERIFIED\033[0m\n")
		fmt.Printf("Verification Source  : %s\n", prof.Source)
		fmt.Printf("Verification Method  : %s\n", prof.VerificationType)
		fmt.Printf("Last Verified At     : %s\n", prof.VerifiedAt.Format("2006-01-02 15:04:05 MST"))
		if prof.NetworkName != "" {
			fmt.Printf("Network Interface    : %s\n", prof.NetworkName)
		}
	} else {
		fmt.Printf("Verification Status  : \033[33mUNVERIFIED\033[0m\n")
		fmt.Printf("Verification Source  : default configuration\n")
		fmt.Printf("Last Verified At     : never\n")
		fmt.Println("\nTip: Run 'sudo discord-bypass tune' to automatically verify on your current network.")
	}
	fmt.Println("============================================================")
}

func cmdStrategySet(args []string) {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: Changing strategy requires administrator privileges.\nPlease run:\n  sudo discord-bypass strategy set %s\n", strings.Join(args, " "))
		os.Exit(1)
	}

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: Missing strategy ID.")
		fmt.Fprintln(os.Stderr, "Usage: sudo discord-bypass strategy set <strategy-id> [--force]")
		fmt.Fprintln(os.Stderr, "Run 'discord-bypass strategy list' to see available strategies.")
		os.Exit(1)
	}

	targetID := args[0]
	force := false
	for _, a := range args[1:] {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	strat, err := strategy.GetStrategy(targetID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfgPath := config.GetActiveConfigPath("")
	prevCfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading configuration: %v\n", err)
		os.Exit(1)
	}
	prevStrategy := prevCfg.General.Strategy

	fmt.Printf("Switching strategy from %q to %q (%s)...\n", prevStrategy, strat.ID, strat.Name)

	// Step 1: Update config file (creates .autobak)
	if err := config.UpdateStrategyInConfig(cfgPath, strat.ID, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update configuration: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Restart service if systemd is active
	if isSystemdRunning() {
		fmt.Println("Restarting discord-bypass service...")
		_ = exec.Command("systemctl", "reset-failed", "discord-bypass").Run()
		if err := exec.Command("systemctl", "restart", "discord-bypass").Run(); err != nil {
			fmt.Printf("[WARN] Service restart failed: %v\n", err)
		}
		time.Sleep(1500 * time.Millisecond)
	}

	// Step 3: Run application-layer connectivity verification
	fmt.Print("Verifying connectivity with new strategy... ")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	probeRes, _ := strategy.VerifyDiscordConnectivity(ctx, 6*time.Second)

	if probeRes != nil && probeRes.Success {
		fmt.Println("\033[32m[PASS]\033[0m")
		fmt.Printf("Connectivity verified: %s\n", probeRes.Details)

		// Record in network profile
		sys, _ := utils.DetectSystem()
		iface := "default"
		if sys != nil && sys.DefaultInterface != "" {
			iface = sys.DefaultInterface
		}
		prof := &strategy.NetworkProfile{
			NetworkName:      iface,
			StrategyID:       strat.ID,
			StrategyName:     strat.Name,
			Status:           "verified",
			VerificationType: "gateway_rest",
			VerifiedAt:       time.Now(),
			Source:           "manual_selection",
		}
		_ = strategy.SaveNetworkProfile(prof)

		fmt.Printf("\033[32m[OK] Strategy successfully applied and verified: %s (%s)\033[0m\n", strat.Name, strat.ID)
		return
	}

	failReason := "Application layer probe failed"
	if probeRes != nil && probeRes.Details != "" {
		failReason = probeRes.Details
	}
	fmt.Printf("\033[31m[FAIL]\033[0m (%s)\n", failReason)

	if force {
		fmt.Println("\033[33m[WARN] Verification failed, but strategy was applied (--force specified).\033[0m")
		prof := &strategy.NetworkProfile{
			StrategyID:       strat.ID,
			StrategyName:     strat.Name,
			Status:           "unverified",
			VerificationType: "manual_forced",
			VerifiedAt:       time.Now(),
			Source:           "manual_selection",
		}
		_ = strategy.SaveNetworkProfile(prof)
		return
	}

	// Rollback
	fmt.Printf("Safely rolling back to previous working strategy (%s)...\n", prevStrategy)
	_ = config.RestoreConfigBackup(cfgPath)
	if isSystemdRunning() {
		_ = exec.Command("systemctl", "restart", "discord-bypass").Run()
	}
	fmt.Printf("[OK] Restored previous strategy (%s). No changes made.\n", prevStrategy)
	fmt.Println("Tip: To force apply without verification, use: sudo discord-bypass strategy set " + strat.ID + " --force")
	os.Exit(1)
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

	// 5. Start periodic runtime status publisher for unprivileged CLI inspection
	statusTicker := time.NewTicker(3 * time.Second)
	statusStop := make(chan struct{})
	startTime := time.Now()

	publishStatus := func() {
		var pkts, bytes uint64
		fwActive := false
		if fw != nil {
			fwActive = fw.IsActive()
			if fwActive {
				pkts, bytes, _ = fw.GetStats()
			}
		}
		beRunning := false
		beName := ""
		if be != nil {
			beRunning = be.IsRunning()
			beName = be.Name()
		}
		st := &utils.RuntimeStatus{
			PID:            os.Getpid(),
			StartTime:      startTime,
			ServiceActive:  true,
			BackendName:    beName,
			BackendRunning: beRunning,
			ActiveStrategy: cfg.General.Strategy,
			StrategyName:   strat.Name,
			FirewallDriver: fw.DriverName(),
			FirewallActive: fwActive,
			Packets:        pkts,
			Bytes:          bytes,
			DNSMode:        cfg.DNS.Mode,
			DoHProvider:    cfg.DNS.DoHProvider,
			SyncHosts:      cfg.DNS.SyncHosts,
			TargetDomains:  len(cfg.Domains),
		}
		_ = utils.WriteRuntimeStatus(st)
	}

	publishStatus()

	go func() {
		for {
			select {
			case <-statusTicker.C:
				publishStatus()
			case <-statusStop:
				return
			}
		}
	}()

	utils.Info("discord-bypass daemon is fully operational and protecting Discord connections")

	// Wait for termination signals (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	statusTicker.Stop()
	close(statusStop)
	utils.RemoveRuntimeStatus()

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
