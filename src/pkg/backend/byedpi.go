package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/utils"
)

type ByeDPIBackend struct {
	cfg       *config.Config
	cmd       *exec.Cmd
	mu        sync.Mutex
	isRunning bool
	cancel    context.CancelFunc
}

func NewByeDPIBackend(cfg *config.Config) *ByeDPIBackend {
	return &ByeDPIBackend{
		cfg: cfg,
	}
}

func (b *ByeDPIBackend) Name() string {
	return "byedpi (ciadpi userspace engine)"
}

func (b *ByeDPIBackend) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.isRunning
}

func (b *ByeDPIBackend) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.isRunning {
		b.mu.Unlock()
		return errors.New("byedpi backend is already running")
	}

	binPath := "ciadpi"
	if _, err := exec.LookPath(binPath); err != nil {
		localBin := "./bin/ciadpi"
		if _, err2 := os.Stat(localBin); err2 == nil {
			binPath = localBin
		} else {
			b.mu.Unlock()
			return fmt.Errorf("ciadpi binary not found in PATH or at %s", localBin)
		}
	}

	subCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.isRunning = true
	b.mu.Unlock()

	args := []string{"-i", "127.0.0.1", "-p", "1080", "--split", "2"}
	cmd := exec.CommandContext(subCtx, binPath, args...)
	b.cmd = cmd

	utils.Info("Starting ByeDPI (ciadpi): %s %v", binPath, args)
	if err := cmd.Start(); err != nil {
		b.mu.Lock()
		b.isRunning = false
		b.mu.Unlock()
		return fmt.Errorf("failed to start ciadpi: %w", err)
	}

	go func() {
		_ = cmd.Wait()
		b.mu.Lock()
		b.isRunning = false
		b.mu.Unlock()
	}()

	return nil
}

func (b *ByeDPIBackend) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.isRunning {
		return nil
	}

	b.isRunning = false
	if b.cancel != nil {
		b.cancel()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
	time.Sleep(100 * time.Millisecond)
	utils.Info("byedpi backend stopped")
	return nil
}
