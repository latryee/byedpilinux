package tests

import (
	"testing"

	"discord-bypass/src/pkg/strategy"
)

func TestGetStrategy(t *testing.T) {
	cases := []struct {
		input           string
		expectedID      string
		requiresBackend bool
		splitPos        int
	}{
		{"strategy_a", "strategy_a", false, 0},
		{"direct", "strategy_a", false, 0},
		{"strategy_b", "strategy_b", false, 0},
		{"dns", "strategy_b", false, 0},
		{"strategy_c", "strategy_c", true, 2},
		{"split2", "strategy_c", true, 2},
		{"segmentation", "strategy_c", true, 2},
		{"strategy_d", "strategy_d", true, 2},
		{"fake", "strategy_d", true, 2},
	}

	for _, tc := range cases {
		strat, err := strategy.GetStrategy(tc.input)
		if err != nil {
			t.Errorf("GetStrategy(%q) failed: %v", tc.input, err)
			continue
		}

		if strat.ID != tc.expectedID {
			t.Errorf("GetStrategy(%q) expected ID %s, got %s", tc.input, tc.expectedID, strat.ID)
		}

		if strat.RequiresBackend != tc.requiresBackend {
			t.Errorf("GetStrategy(%q) expected RequiresBackend %v, got %v", tc.input, tc.requiresBackend, strat.RequiresBackend)
		}

		if strat.NativeSplitPos != tc.splitPos {
			t.Errorf("GetStrategy(%q) expected NativeSplitPos %d, got %d", tc.input, tc.splitPos, strat.NativeSplitPos)
		}
	}
}

func TestInvalidStrategy(t *testing.T) {
	_, err := strategy.GetStrategy("nonexistent_strategy")
	if err == nil {
		t.Errorf("Expected error for invalid strategy, got nil")
	}
}
