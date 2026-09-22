package strategy

import (
	"fmt"
	"strings"
)

type Strategy struct {
	ID              string
	Name            string
	Description     string
	RequiresBackend bool
	RequiresDoH     bool
	NFQWSArgs       []string
	NativeSplitPos  int
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
	},
	"strategy_b": {
		ID:              "strategy_b",
		Name:            "Secure DNS Workaround",
		Description:     "Bypasses standard port 53 DNS poisoning/hijacking using DNS over HTTPS.",
		RequiresBackend: false,
		RequiresDoH:     true,
		NFQWSArgs:       []string{},
		NativeSplitPos:  0,
	},
	"strategy_c": {
		ID:              "strategy_c",
		Name:            "TLS SNI Split",
		Description:     "Uses zapret's TLS ClientHello split at byte 2. Its network effectiveness is unverified until live-tested.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=multisplit", "--dpi-desync-split-pos=2"},
		NativeSplitPos:  2,
	},
	"strategy_d": {
		ID:              "strategy_d",
		Name:            "Fake + Split",
		Description:     "Uses a fake TLS packet followed by a split ClientHello. Its network effectiveness is unverified until live-tested.",
		RequiresBackend: true,
		RequiresDoH:     true,
		NFQWSArgs:       []string{"--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=2", "--dpi-desync-ttl=4"},
		NativeSplitPos:  2,
	},
}

func GetStrategy(id string) (Strategy, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	// Alias support
	switch key {
	case "a", "direct":
		key = "strategy_a"
	case "b", "dns":
		key = "strategy_b"
	case "c", "split", "split2", "segmentation":
		key = "strategy_c"
	case "d", "fake", "advanced":
		key = "strategy_d"
	}

	strat, ok := AvailableStrategies[key]
	if !ok {
		return Strategy{}, fmt.Errorf("unknown strategy: %s (available: strategy_a, strategy_b, strategy_c, strategy_d)", id)
	}
	return strat, nil
}
