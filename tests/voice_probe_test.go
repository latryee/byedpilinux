package tests

import (
	"context"
	"net"
	"testing"
	"time"

	"discord-bypass/src/pkg/strategy"
)

func TestVoiceUDPProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "udp", "latency.discord.media:443")
	if err != nil {
		t.Logf("Voice UDP socket dial returned (non-fatal if offline/firewalled): %v", err)
		return
	}
	defer conn.Close()

	// STUN packet probe
	n, err := conn.Write([]byte{0x00, 0x01, 0x00, 0x00})
	if err != nil {
		t.Logf("Voice UDP write failed: %v", err)
		return
	}
	if n != 4 {
		t.Errorf("expected 4 bytes written, got %d", n)
	}
}

func TestVerifierResultVoiceField(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := strategy.VerifyDiscordConnectivity(ctx, 4*time.Second)
	if err != nil {
		t.Fatalf("unexpected error from verifier: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil verification result")
	}

	t.Logf("Verification Result: Success=%v, VoiceReady=%v, Latency=%v", res.Success, res.VoiceReady, res.Latency)
}
