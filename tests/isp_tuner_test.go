package tests

import (
	"testing"

	"discord-bypass/src/pkg/diagnostics"
	"discord-bypass/src/pkg/strategy"
)

func TestPrioritizeCandidatesForSuperonline(t *testing.T) {
	isp := &diagnostics.ISPDiagnosticResult{
		DetectedISP: "Turkcell Superonline",
		ASN:         "AS34984",
		Country:     "Turkey",
	}

	candidates := strategy.CandidateStrategies()
	reordered := strategy.PrioritizeCandidatesForISP(isp, candidates)

	if len(reordered) != len(candidates) {
		t.Fatalf("expected %d candidates, got %d", len(candidates), len(reordered))
	}

	// Superonline must have strategy_c at the very top
	if reordered[0].ID != "strategy_c" {
		t.Errorf("expected top candidate for Superonline to be strategy_c, got %s", reordered[0].ID)
	}

	// Strategy D should follow
	if reordered[1].ID != "strategy_d" {
		t.Errorf("expected second candidate for Superonline to be strategy_d, got %s", reordered[1].ID)
	}
}

func TestPrioritizeCandidatesForTurkTelekom(t *testing.T) {
	isp := &diagnostics.ISPDiagnosticResult{
		DetectedISP: "Turk Telekom",
		ASN:         "AS9121",
		Country:     "Turkey",
	}

	candidates := strategy.CandidateStrategies()
	reordered := strategy.PrioritizeCandidatesForISP(isp, candidates)

	if len(reordered) != len(candidates) {
		t.Fatalf("expected %d candidates, got %d", len(candidates), len(reordered))
	}

	if reordered[0].ID != "strategy_c_ttl5" {
		t.Errorf("expected top candidate for Turk Telekom to be strategy_c_ttl5, got %s", reordered[0].ID)
	}
}

func TestPrioritizeCandidatesForTurkNet(t *testing.T) {
	isp := &diagnostics.ISPDiagnosticResult{
		DetectedISP: "TurkNet Iletisim",
		ASN:         "AS12735",
		Country:     "Turkey",
	}

	candidates := strategy.CandidateStrategies()
	reordered := strategy.PrioritizeCandidatesForISP(isp, candidates)

	if len(reordered) != len(candidates) {
		t.Fatalf("expected %d candidates, got %d", len(candidates), len(reordered))
	}

	if reordered[0].ID != "strategy_b" {
		t.Errorf("expected top candidate for TurkNet to be strategy_b, got %s", reordered[0].ID)
	}
}

func TestPrioritizeCandidatesNilISP(t *testing.T) {
	candidates := strategy.CandidateStrategies()
	reordered := strategy.PrioritizeCandidatesForISP(nil, candidates)

	if len(reordered) != len(candidates) {
		t.Fatalf("expected %d candidates, got %d", len(candidates), len(reordered))
	}

	for i := range candidates {
		if reordered[i].ID != candidates[i].ID {
			t.Errorf("candidate[%d] mismatch with nil ISP: got %s, want %s", i, reordered[i].ID, candidates[i].ID)
		}
	}
}
