package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"discord-bypass/src/pkg/backend"
	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/firewall"
)

func TestNFQWSBuildArgsValidation(t *testing.T) {
	// Find binary
	binPath := "../bin/discord-bypass-nfqws"
	if _, err := os.Stat(binPath); err != nil {
		binPath = "bin/discord-bypass-nfqws"
		if _, err := os.Stat(binPath); err != nil {
			t.Skip("nfqws binary not yet built, skipping dry-run validation")
		}
	}
	absBin, _ := filepath.Abs(binPath)

	domainsPath := "../config/domains.txt"
	if _, err := os.Stat(domainsPath); err != nil {
		domainsPath = "config/domains.txt"
	}
	absDomains, _ := filepath.Abs(domainsPath)

	testCases := []struct {
		name     string
		strategy string
		cArgs    []string
		dArgs    []string
	}{
		{
			name:     "strategy_c (multisplit split-pos=2)",
			strategy: "strategy_c",
			cArgs:    []string{"--dpi-desync=multisplit", "--dpi-desync-split-pos=2"},
			dArgs:    nil,
		},
		{
			name:     "strategy_d (fake,multisplit ttl=4)",
			strategy: "strategy_d",
			cArgs:    nil,
			dArgs:    []string{"--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=2", "--dpi-desync-ttl=4"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				General: config.GeneralConfig{
					Strategy:    tc.strategy,
					DomainsFile: absDomains,
				},
				Firewall: config.FirewallConfig{
					QueueNum: 200,
				},
				NFQWS: config.NFQWSConfig{
					BinaryPath:     absBin,
					StrategyCArgs: tc.cArgs,
					StrategyDArgs: tc.dArgs,
				},
			}

			be := backend.NewNFQWSBackend(cfg)
			args := be.BuildArgs()

			// 1. Verify queue number
			hasQnum := false
			for _, a := range args {
				if a == "--qnum=200" {
					hasQnum = true
				}
			}
			if !hasQnum {
				t.Errorf("args missing --qnum=200: %v", args)
			}

			// 2. Verify loop-prevention mark
			hasMark := false
			for _, a := range args {
				if a == "--dpi-desync-fwmark=0x40000000" {
					hasMark = true
				}
			}
			if !hasMark {
				t.Errorf("args missing critical loop-prevention fwmark: %v", args)
			}

			// 3. Verify dry-run with actual compiled nfqws engine
			cmdArgs := append(args, "--dry-run")
			cmd := exec.Command(absBin, cmdArgs...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("nfqws --dry-run failed with args %v: %s (%v)", cmdArgs, string(out), err)
			}

			if !strings.Contains(string(out), "command line parameters verified") {
				t.Errorf("expected 'command line parameters verified' in nfqws output, got: %s", string(out))
			}
		})
	}
}

func TestNFTablesScriptOrderingAndLoopPrevention(t *testing.T) {
	mgr := firewall.NewNFTablesManager("discord_bypass", true)

	script, err := mgr.BuildScript("nfqueue", 200, 10443, false)
	if err != nil {
		t.Fatalf("BuildScript failed: %v", err)
	}

	// 1. Verify dedicated table and sets
	if !strings.Contains(script, "add table inet discord_bypass") {
		t.Error("missing add table statement")
	}
	if !strings.Contains(script, "add set inet discord_bypass discord_v4") {
		t.Error("missing discord_v4 set definition")
	}
	if !strings.Contains(script, "add set inet discord_bypass discord_v6") {
		t.Error("missing discord_v6 set definition")
	}

	// 2. Critical Loop Prevention: meta mark 0x40000000 accept MUST appear before queue
	markRule := "add rule inet discord_bypass output meta mark 0x40000000 counter accept"
	queueRuleV4 := "add rule inet discord_bypass output ip daddr @discord_v4 tcp dport 443 counter queue num 200 bypass"
	queueRuleV6 := "add rule inet discord_bypass output ip6 daddr @discord_v6 tcp dport 443 counter queue num 200 bypass"

	idxMark := strings.Index(script, markRule)
	idxQueueV4 := strings.Index(script, queueRuleV4)
	idxQueueV6 := strings.Index(script, queueRuleV6)

	if idxMark == -1 {
		t.Fatalf("script missing critical loop-prevention rule: %q", markRule)
	}
	if idxQueueV4 == -1 {
		t.Fatalf("script missing IPv4 queue rule: %q", queueRuleV4)
	}
	if idxQueueV6 == -1 {
		t.Fatalf("script missing IPv6 queue rule: %q", queueRuleV6)
	}

	if idxMark >= idxQueueV4 {
		t.Fatalf("CRITICAL BUG: fwmark accept rule (pos %d) MUST appear BEFORE IPv4 NFQUEUE rule (pos %d) to prevent packet loops!",
			idxMark, idxQueueV4)
	}
	if idxMark >= idxQueueV6 {
		t.Fatalf("CRITICAL BUG: fwmark accept rule (pos %d) MUST appear BEFORE IPv6 NFQUEUE rule (pos %d) to prevent packet loops!",
			idxMark, idxQueueV6)
	}

	// 3. Test blockQUIC
	scriptWithQUIC, err := mgr.BuildScript("nfqueue", 200, 10443, true)
	if err != nil {
		t.Fatalf("BuildScript with blockQUIC failed: %v", err)
	}
	if !strings.Contains(scriptWithQUIC, "udp dport 443 counter drop") {
		t.Errorf("expected QUIC drop rule when blockQUIC is true")
	}

	// 4. Validate syntax with nft -c via unshare namespace if nft is present
	if _, err := exec.LookPath("nft"); err == nil {
		validateCmd := exec.Command("unshare", "-r", "-n", "nft", "-c", "-f", "-")
		validateCmd.Stdin = strings.NewReader(script)
		out, err := validateCmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "Operation not permitted") || strings.Contains(string(out), "unshare") {
				t.Skip("network namespaces are unavailable in this test environment")
			}
			t.Fatalf("nft -c failed on generated script: %s (%v)", string(out), err)
		}
	}
}
