package strategy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/diagnostics"
	"discord-bypass/src/pkg/utils"
)

// TuningResult summarizes the outcome of the automatic strategy tuner.
type TuningResult struct {
	Success          bool                `json:"success"`
	SelectedStrategy Strategy            `json:"selected_strategy"`
	Verification     *VerificationResult `json:"verification,omitempty"`
	CandidatesTested int                 `json:"candidates_tested"`
	PreviousStrategy string              `json:"previous_strategy"`
	Message          string              `json:"message"`
}

// Tuner orchestrates candidate evaluation, safe testing, and transactional rollback.
type Tuner struct {
	ConfigPath string
	Verbose    bool
}

// NewTuner creates a new Tuner instance.
func NewTuner(configPath string, verbose bool) *Tuner {
	return &Tuner{
		ConfigPath: configPath,
		Verbose:    verbose,
	}
}

// Run executes the automatic strategy tuning lifecycle.
func (t *Tuner) Run(ctx context.Context) (*TuningResult, error) {
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("automatic strategy tuning requires root privileges. Please run with sudo:\n  sudo discord-bypass tune")
	}

	cfgPath := config.GetActiveConfigPath(t.ConfigPath)
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	prevStrategy := cfg.General.Strategy
	sys, _ := utils.DetectSystem()
	ifaceName := "default"
	if sys != nil && sys.DefaultInterface != "" {
		ifaceName = sys.DefaultInterface
	}

	fmt.Println("============================================================")
	fmt.Println("               Discord Bypass Strategy Tuner                ")
	fmt.Println("============================================================")
	fmt.Printf("Current network interface : %s\n", ifaceName)
	fmt.Printf("Active configuration      : %s\n", cfgPath)
	fmt.Printf("Previous strategy         : %s\n", prevStrategy)

	ispInfo, _ := diagnostics.DetectISP(ctx)
	if ispInfo != nil && ispInfo.DetectedISP != "" {
		fmt.Printf("Detected ISP              : %s (%s, %s)\n", ispInfo.DetectedISP, ispInfo.ASN, ispInfo.Country)
	}
	fmt.Println("------------------------------------------------------------")

	// Setup emergency signal trap for safe rollback if interrupted (Ctrl+C / SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	tuningComplete := false
	defer func() {
		signal.Stop(sigChan)
		if !tuningComplete {
			// Restore previous configuration
			_ = config.RestoreConfigBackup(cfgPath)
			_ = t.restartService()
		}
	}()

	go func() {
		select {
		case <-sigChan:
			fmt.Println("\n[!] Tuning interrupted by user. Safely rolling back to previous configuration...")
			_ = config.RestoreConfigBackup(cfgPath)
			_ = t.restartService()
			fmt.Println("[OK] Rollback complete. Exiting.")
			os.Exit(130)
		case <-ctx.Done():
			return
		}
	}()

	// 1. Initial Direct Connectivity Test
	// Determine whether Discord is actually blocked on this network
	fmt.Println("[Step 1/2] Checking direct Discord reachability...")
	_ = config.UpdateStrategyInConfig(cfgPath, "strategy_a", nil)
	_ = t.restartService()
	time.Sleep(1200 * time.Millisecond)

	directRes, _ := VerifyDiscordConnectivity(ctx, 4*time.Second)
	if directRes != nil && directRes.Success {
		fmt.Printf("  \033[32m[OK]\033[0m Direct Discord connectivity is fully functional!\n")
		fmt.Println("       DPI circumvention is not required on this network.")
		tuningComplete = true
		_ = config.UpdateStrategyInConfig(cfgPath, "strategy_a", nil)
		_ = t.restartService()

		prof := &NetworkProfile{
			NetworkName:      ifaceName,
			StrategyID:       "strategy_a",
			StrategyName:     "Direct (Baseline)",
			Status:           "verified",
			VerificationType: "direct",
			VerifiedAt:       time.Now(),
			Source:           "automatic_tuning",
		}
		_ = SaveNetworkProfile(prof)

		stratA, _ := GetStrategy("strategy_a")
		return &TuningResult{
			Success:          true,
			SelectedStrategy: stratA,
			Verification:     directRes,
			CandidatesTested: 1,
			PreviousStrategy: prevStrategy,
			Message:          "Direct connectivity working cleanly; no DPI bypass required.",
		}, nil
	}

	fmt.Println("  \033[33m[INFO]\033[0m Direct connection is blocked or restricted. Testing bypass candidates...")
	fmt.Println("\n[Step 2/2] Evaluating candidate strategies for this network...")

	// Candidates ordered by detected Turkish ISP priority and CandidateRank
	candidates := PrioritizeCandidatesForISP(ispInfo, CandidateStrategies())
	testedCount := 0

	for idx, cand := range candidates {
		// Skip direct (already tested in step 1)
		if cand.ID == "strategy_a" {
			continue
		}

		testedCount++
		fmt.Printf("\n[%d/%d] Candidate: %s (%s)\n", idx+1, len(candidates), cand.Name, cand.ID)
		if len(cand.NFQWSArgs) > 0 {
			fmt.Printf("      NFQWS Args: %s\n", strings.Join(cand.NFQWSArgs, " "))
		}

		// Apply candidate
		if err := config.UpdateStrategyInConfig(cfgPath, cand.ID, nil); err != nil {
			fmt.Printf("      \033[31m[FAIL]\033[0m Failed to apply candidate config: %v\n", err)
			continue
		}

		// Restart service
		if err := t.restartService(); err != nil {
			fmt.Printf("      \033[31m[FAIL]\033[0m Failed to restart bypass service: %v\n", err)
			continue
		}

		// Wait for nfqws to hook netfilter
		time.Sleep(1500 * time.Millisecond)

		// Probe connectivity
		probeStart := time.Now()
		probeRes, _ := VerifyDiscordConnectivity(ctx, 5*time.Second)
		elapsed := time.Since(probeStart)

		if probeRes != nil && probeRes.Success {
			fmt.Printf("      \033[32m[PASS]\033[0m %s (latency: %v)\n", probeRes.Details, elapsed)
			fmt.Printf("\n\033[32m============================================================\033[0m\n")
			fmt.Printf("\033[32m  VERIFIED WORKING STRATEGY FOUND: %s (%s)\033[0m\n", cand.Name, cand.ID)
			fmt.Printf("\033[32m============================================================\033[0m\n")

			tuningComplete = true
			// Persist permanently
			_ = config.UpdateStrategyInConfig(cfgPath, cand.ID, nil)

			// Update network profile
			prof := &NetworkProfile{
				NetworkName:      ifaceName,
				StrategyID:       cand.ID,
				StrategyName:     cand.Name,
				Status:           "verified",
				VerificationType: "gateway_rest",
				VerifiedAt:       time.Now(),
				Source:           "automatic_tuning",
			}
			_ = SaveNetworkProfile(prof)

			// Clean service restart with verified configuration
			_ = t.restartService()

			fmt.Printf("Strategy successfully persisted to %s\n", cfgPath)
			fmt.Printf("Network profile updated at %s\n", GetProfilePath())
			fmt.Println("\nDiscord Bypass is active and ready.")

			return &TuningResult{
				Success:          true,
				SelectedStrategy: cand,
				Verification:     probeRes,
				CandidatesTested: testedCount,
				PreviousStrategy: prevStrategy,
				Message:          fmt.Sprintf("Verified working strategy: %s (%s)", cand.Name, cand.ID),
			}, nil
		}

		failReason := "Probe failed"
		if probeRes != nil && probeRes.Details != "" {
			failReason = probeRes.Details
		}
		fmt.Printf("      \033[31m[FAIL]\033[0m %s\n", failReason)
	}

	// If we reach here, all candidates failed
	fmt.Printf("\n\033[31m============================================================\033[0m\n")
	fmt.Println("\033[31m[FAIL] All candidate strategies failed to establish verified connectivity.\033[0m")
	fmt.Printf("Restoring previous configuration (%s)...\n", prevStrategy)
	_ = config.RestoreConfigBackup(cfgPath)
	_ = t.restartService()
	tuningComplete = true // prevent defer from double-restoring
	fmt.Println("[OK] Restored previous configuration.")
	fmt.Println("Run 'discord-bypass diagnose' to inspect low-level network blocks.")
	fmt.Printf("\033[31m============================================================\033[0m\n")

	if ispInfo != nil && (strings.Contains(strings.ToUpper(ispInfo.ASN), "34984") || strings.Contains(strings.ToUpper(ispInfo.DetectedISP), "SUPERONLINE") || strings.Contains(strings.ToUpper(ispInfo.ASN), "9121") || strings.Contains(strings.ToUpper(ispInfo.DetectedISP), "TELEKOM")) {
		fmt.Printf("\n\033[33m============================================================\033[0m\n")
		fmt.Println("\033[33m [!] DİKKAT: TÜRKİYE ISS GÜVENLİ İNTERNET PROFİLİ UYARISI   \033[0m")
		fmt.Println("------------------------------------------------------------")
		fmt.Printf("Hattınızda (%s) test edilen tüm DPI stratejileri başarısız oldu.\n", ispInfo.DetectedISP)
		fmt.Println("Bu durum büyük olasılıkla ISS hesabınızda 'Güvenli İnternet' (Aile/Çocuk)")
		fmt.Println("profilinin açık olmasından kaynaklanır. Bu profil, paketleri DPI")
		fmt.Println("öncesinde doğrudan santral (BRAS) seviyesinde sıfırlar (TCP RST).")
		fmt.Println("")
		fmt.Println("ÇÖZÜM ADIMLARI:")
		fmt.Println(" 1. ISS Online İşlemler web sitesine veya mobil uygulamasına girin.")
		fmt.Println(" 2. 'Güvenli İnternet' ayarlarından profilinizi 'STANDART PROFİL' yapın.")
		fmt.Println(" 3. Modeminizi kapatıp açın.")
		fmt.Println(" 4. 'sudo discord-bypass tune' komutunu tekrar çalıştırın.")
		fmt.Printf("\033[33m============================================================\033[0m\n")
	}

	prevStratObj, _ := GetStrategy(prevStrategy)
	return &TuningResult{
		Success:          false,
		SelectedStrategy: prevStratObj,
		CandidatesTested: testedCount,
		PreviousStrategy: prevStrategy,
		Message:          "All candidates failed connectivity verification; previous configuration restored.",
	}, fmt.Errorf("all %d candidate strategies failed verification on this network", testedCount)
}

func (t *Tuner) restartService() error {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		_ = exec.Command("systemctl", "reset-failed", "discord-bypass").Run()
		return exec.Command("systemctl", "restart", "discord-bypass").Run()
	}
	return nil
}

// PrioritizeCandidatesForISP reorders candidate strategies based on the detected ISP in Turkey.
func PrioritizeCandidatesForISP(isp *diagnostics.ISPDiagnosticResult, candidates []Strategy) []Strategy {
	if isp == nil {
		return candidates
	}

	asUpper := strings.ToUpper(isp.ASN)
	ispUpper := strings.ToUpper(isp.DetectedISP)

	var preferredOrder []string
	if strings.Contains(asUpper, "34984") || strings.Contains(ispUpper, "SUPERONLINE") || strings.Contains(ispUpper, "TURKCELL") {
		// Turkcell Superonline: strategy_c (fakedsplit midsld ttl=6) is confirmed 100% effective
		preferredOrder = []string{"strategy_c", "strategy_d", "strategy_c_ttl5", "strategy_c_ttl4", "strategy_fake_multisplit", "strategy_split2"}
	} else if strings.Contains(asUpper, "9121") || strings.Contains(ispUpper, "TURK TELEKOM") || strings.Contains(ispUpper, "TTNET") {
		// Türk Telekom: strategy_c_ttl5, strategy_c, strategy_b (DoH)
		preferredOrder = []string{"strategy_c_ttl5", "strategy_c", "strategy_b", "strategy_d_ttl5", "strategy_d", "strategy_split2"}
	} else if strings.Contains(asUpper, "12735") || strings.Contains(ispUpper, "TURKNET") {
		// TurkNet: strategy_b, strategy_c
		preferredOrder = []string{"strategy_b", "strategy_c", "strategy_c_ttl5", "strategy_split2"}
	} else if strings.Contains(asUpper, "15897") || strings.Contains(ispUpper, "VODAFONE") {
		// Vodafone Net: strategy_c, strategy_d
		preferredOrder = []string{"strategy_c", "strategy_d", "strategy_c_ttl5", "strategy_b"}
	} else if strings.Contains(asUpper, "47524") || strings.Contains(ispUpper, "KABLONET") || strings.Contains(ispUpper, "TURKSAT") {
		// Türksat Kablonet: strategy_c, strategy_split2
		preferredOrder = []string{"strategy_c", "strategy_split2", "strategy_c_ttl5", "strategy_b"}
	}

	if len(preferredOrder) == 0 {
		return candidates
	}

	candMap := make(map[string]Strategy)
	for _, c := range candidates {
		candMap[c.ID] = c
	}

	var reordered []Strategy
	used := make(map[string]bool)

	for _, id := range preferredOrder {
		if cand, exists := candMap[id]; exists {
			reordered = append(reordered, cand)
			used[id] = true
		}
	}

	// Append any remaining candidates preserving original relative order
	for _, c := range candidates {
		if !used[c.ID] {
			reordered = append(reordered, c)
		}
	}

	return reordered
}
