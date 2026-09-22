package tests

import (
	"path/filepath"
	"testing"
	"time"

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
		{"doh", "strategy_b", false, 0},
		{"strategy_c", "strategy_c", true, 2},
		{"fakedsplit", "strategy_c", true, 2},
		{"strategy_c_ttl5", "strategy_c_ttl5", true, 2},
		{"strategy_c_ttl4", "strategy_c_ttl4", true, 2},
		{"strategy_d", "strategy_d", true, 2},
		{"fakeddisorder", "strategy_d", true, 2},
		{"strategy_d_ttl5", "strategy_d_ttl5", true, 2},
		{"strategy_split2", "strategy_split2", true, 2},
		{"split2", "strategy_split2", true, 2},
		{"strategy_fake_multisplit", "strategy_fake_multisplit", true, 2},
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

func TestCandidateStrategies(t *testing.T) {
	candidates := strategy.CandidateStrategies()
	if len(candidates) < 5 {
		t.Fatalf("Expected at least 5 candidates, got %d", len(candidates))
	}

	// Verify order: candidate_rank must be strictly increasing
	for i := 1; i < len(candidates); i++ {
		if candidates[i].CandidateRank <= candidates[i-1].CandidateRank {
			t.Errorf("Candidate ordering violation at index %d: rank %d <= %d",
				i, candidates[i].CandidateRank, candidates[i-1].CandidateRank)
		}
	}

	// Candidate 1 must be direct (strategy_a)
	if candidates[0].ID != "strategy_a" {
		t.Errorf("First candidate must be strategy_a, got %s", candidates[0].ID)
	}

	// Candidate 2 must be strategy_c (verified on Superonline)
	if candidates[1].ID != "strategy_c" {
		t.Errorf("Second candidate must be strategy_c, got %s", candidates[1].ID)
	}
}

func TestListStrategies(t *testing.T) {
	strats := strategy.ListStrategies()
	if len(strats) < 8 {
		t.Fatalf("Expected at least 8 strategies in list, got %d", len(strats))
	}

	// Verify sorted by ID
	for i := 1; i < len(strats); i++ {
		if strats[i].ID < strats[i-1].ID {
			t.Errorf("Strategies not sorted: %s before %s", strats[i].ID, strats[i-1].ID)
		}
	}
}

func TestNetworkProfileSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	profPath := filepath.Join(tmpDir, "profile.json")

	now := time.Now().Truncate(time.Second)
	orig := &strategy.NetworkProfile{
		NetworkName:      "test_interface",
		StrategyID:       "strategy_c",
		StrategyName:     "TLS SNI Fake Split (fakedsplit midsld ttl=6)",
		Status:           "verified",
		VerificationType: "gateway_rest",
		VerifiedAt:       now,
		Source:           "automatic_tuning",
	}

	if err := strategy.SaveNetworkProfileTo(profPath, orig); err != nil {
		t.Fatalf("SaveNetworkProfileTo failed: %v", err)
	}

	loaded, err := strategy.LoadNetworkProfileFrom(profPath)
	if err != nil {
		t.Fatalf("LoadNetworkProfileFrom failed: %v", err)
	}

	if loaded.StrategyID != orig.StrategyID || loaded.Status != "verified" || loaded.Source != orig.Source {
		t.Errorf("Loaded profile mismatch: got %+v, want %+v", loaded, orig)
	}
}

func TestInvalidStrategy(t *testing.T) {
	_, err := strategy.GetStrategy("nonexistent_strategy")
	if err == nil {
		t.Errorf("Expected error for invalid strategy, got nil")
	}
}
