package tests

import (
	"context"
	"testing"
	"time"

	"discord-bypass/src/pkg/strategy"
)

func TestVerifyDiscordConnectivityLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := strategy.VerifyDiscordConnectivity(ctx, 8*time.Second)
	if err != nil {
		t.Fatalf("VerifyDiscordConnectivity error: %v", err)
	}

	t.Logf("Live Verification Result: Success=%v, DNSClean=%v, TCPConnected=%v, TLSNegotiated=%v, ApplicationLayer=%v",
		res.Success, res.DNSClean, res.TCPConnected, res.TLSNegotiated, res.ApplicationLayer)
	t.Logf("Details: %s", res.Details)
	t.Logf("Latency: %v, RemoteAddress: %s", res.Latency, res.RemoteAddress)

	if !res.DNSClean {
		t.Errorf("Expected clean DNS on live network")
	}
	if !res.TCPConnected {
		t.Errorf("Expected TCP connected on live network")
	}
	if !res.TLSNegotiated {
		t.Errorf("Expected TLS negotiated on live network with working bypass")
	}
	if !res.ApplicationLayer {
		t.Errorf("Expected ApplicationLayer verified on live network with working bypass")
	}
	if !res.Success {
		t.Errorf("Expected overall verification Success=true on live network")
	}
}

func TestVerifyTimeoutHandling(t *testing.T) {
	// A cancelled context must gracefully fail without hanging or panicking
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	res, err := strategy.VerifyDiscordConnectivity(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected nil error for context cancellation, got: %v", err)
	}
	if res.Success {
		t.Errorf("Expected Success=false for cancelled context, got true")
	}
	if res.FailedStage == "" {
		t.Errorf("Expected FailedStage to be populated on failure")
	}
}
