package strategy

import (
	"fmt"
	"sort"
	"strings"
)

type Strategy struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	RequiresBackend bool     `json:"requires_backend"`
	RequiresDoH     bool     `json:"requires_doh"`
	NFQWSArgs       []string `json:"nfqws_args"`
	NativeSplitPos  int      `json:"native_split_pos"`
	CandidateRank   int      `json:"candidate_rank"` // Order for automatic tuning (0 = not an auto candidate)
}

var AvailableStrategies = map[string]Strategy{
	"strategy_a": {
		ID:              "strategy_a",
		Name:            "Direct (Baseline)",
		Description:     "Direct connection without DPI manipulation or DoH. Tests baseline ISP behavior.",
		RequiresBackend: false,
		RequiresDoH:     false,
		NFQWSArgs:       []string{},
		NativeSplitPos:  0,
		CandidateRank:   1, // Tested first in tune
	},
	"strategy_b": {
		ID:              "strategy_b",
		Name:            "Secure DNS Workaround",
		Description:     "Bypasses standard port 53 DNS poisoning/hijacking using DNS over HTTPS without packet manipulation.",
		RequiresBackend: false,
		RequiresDoH:     true,
		NFQWSArgs:       []string{},
		NativeSplitPos:  0,
		CandidateRank:   7,
	},
	"strategy_c": {
		ID:              "strategy_c",
		Name:            "TLS SNI Fake Split (fakedsplit midsld ttl=6)",
		Description:     "Splits the TLS ClientHello at midsld with a TTL=6 fake segment. Verified effective on Turkcell Superonline and major Turkish ISPs.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fakedsplit", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=6"},
		NativeSplitPos:  2,
		CandidateRank:   2, // Top priority DPI candidate
	},
	"strategy_c_ttl5": {
		ID:              "strategy_c_ttl5",
		Name:            "TLS SNI Fake Split (fakedsplit midsld ttl=5)",
		Description:     "Splits the TLS ClientHello at midsld with a TTL=5 fake segment. Alternative TTL for shorter ISP transit paths.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fakedsplit", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=5"},
		NativeSplitPos:  2,
		CandidateRank:   3,
	},
	"strategy_c_ttl4": {
		ID:              "strategy_c_ttl4",
		Name:            "TLS SNI Fake Split (fakedsplit midsld ttl=4)",
		Description:     "Splits the TLS ClientHello at midsld with a TTL=4 fake segment. Effective for low-hop ISP middleboxes.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fakedsplit", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=4"},
		NativeSplitPos:  2,
		CandidateRank:   4,
	},
	"strategy_d": {
		ID:              "strategy_d",
		Name:            "Advanced Out-of-Order Desync (fakeddisorder midsld ttl=6)",
		Description:     "Sends an out-of-order fake TLS segment at midsld with TTL=6. Resilient fallback for stateful middleboxes with aggressive TCP reassembly.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fakeddisorder", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=6"},
		NativeSplitPos:  2,
		CandidateRank:   5,
	},
	"strategy_d_ttl5": {
		ID:              "strategy_d_ttl5",
		Name:            "Advanced Out-of-Order Desync (fakeddisorder midsld ttl=5)",
		Description:     "Sends an out-of-order fake TLS segment at midsld with TTL=5.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fakeddisorder", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=5"},
		NativeSplitPos:  2,
		CandidateRank:   6,
	},
	"strategy_split2": {
		ID:              "strategy_split2",
		Name:            "Standard TCP Segmentation (multisplit split-pos=2)",
		Description:     "Splits the TLS ClientHello at byte 2 without fake packets. Effective against simpler stateless DPIs.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=multisplit", "--dpi-desync-split-pos=2"},
		NativeSplitPos:  2,
		CandidateRank:   8,
	},
	"strategy_fake_multisplit": {
		ID:              "strategy_fake_multisplit",
		Name:            "Full Fake ClientHello + Multisplit (midsld, ttl=5)",
		Description:     "Injects a full fake TLS ClientHello followed by multisplit at midsld with TTL=5.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=midsld", "--dpi-desync-ttl=5"},
		NativeSplitPos:  2,
		CandidateRank:   9,
	},
}

// GetStrategy resolves a strategy ID or alias into its Strategy definition.
func GetStrategy(id string) (Strategy, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	// Alias mapping
	switch key {
	case "a", "direct":
		key = "strategy_a"
	case "b", "dns", "doh":
		key = "strategy_b"
	case "c", "split", "fakedsplit", "fakedsplit_midsld_ttl6":
		key = "strategy_c"
	case "c_ttl5", "fakedsplit_midsld_ttl5":
		key = "strategy_c_ttl5"
	case "c_ttl4", "fakedsplit_midsld_ttl4":
		key = "strategy_c_ttl4"
	case "d", "fake", "advanced", "fakeddisorder", "fakeddisorder_midsld_ttl6":
		key = "strategy_d"
	case "d_ttl5", "fakeddisorder_midsld_ttl5":
		key = "strategy_d_ttl5"
	case "split2", "multisplit":
		key = "strategy_split2"
	case "fake_multisplit", "fakemultisplit":
		key = "strategy_fake_multisplit"
	}

	strat, ok := AvailableStrategies[key]
	if !ok {
		return Strategy{}, fmt.Errorf("unknown strategy: %q (run 'discord-bypass strategy list' for available strategies)", id)
	}
	return strat, nil
}

// ListStrategies returns all available strategies sorted alphabetically by ID.
func ListStrategies() []Strategy {
	var list []Strategy
	for _, s := range AvailableStrategies {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

// CandidateStrategies returns candidate strategies ordered by their candidate ranking for auto-tuning.
func CandidateStrategies() []Strategy {
	var candidates []Strategy
	for _, s := range AvailableStrategies {
		if s.CandidateRank > 0 {
			candidates = append(candidates, s)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CandidateRank < candidates[j].CandidateRank
	})
	return candidates
}
