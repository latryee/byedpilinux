package backend

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"discord-bypass/src/pkg/config"
	"discord-bypass/src/pkg/utils"
)

type NFQWSBackend struct {
	cfg       *config.Config
	cmd       *exec.Cmd
	mu        sync.Mutex
	isRunning bool
	cancel    context.CancelFunc
}

func NewNFQWSBackend(cfg *config.Config) *NFQWSBackend {
	return &NFQWSBackend{
		cfg: cfg,
	}
}

func (b *NFQWSBackend) Name() string {
	return "nfqws (Netfilter queue)"
}

func (b *NFQWSBackend) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.isRunning
}

func (b *NFQWSBackend) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.isRunning {
		b.mu.Unlock()
		return errors.New("nfqws backend is already running")
	}

	binPath := b.cfg.NFQWS.BinaryPath
	if _, err := exec.LookPath(binPath); err != nil {
		// Fallback check in local build dir
		localBin := "./bin/discord-bypass-nfqws"
		if _, err2 := os.Stat(localBin); err2 == nil {
			binPath = localBin
		} else {
			b.mu.Unlock()
			return fmt.Errorf("nfqws binary not found at %s or %s (run 'make build' or 'discord-bypass install')", binPath, localBin)
		}
	}

	subCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.isRunning = true
	b.mu.Unlock()

	go b.supervisorLoop(subCtx, binPath)
	return nil
}

func (b *NFQWSBackend) Stop() error {
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
		_ = b.cmd.Process.Signal(os.Interrupt)
		time.Sleep(200 * time.Millisecond)
		_ = b.cmd.Process.Kill()
	}

	utils.Info("nfqws backend stopped")
	return nil
}

func (b *NFQWSBackend) buildArgs() []string {
	args := []string{
		"--qnum=" + strconv.Itoa(b.cfg.Firewall.QueueNum),
	}

	// Hostlist: only desync traffic for targeted domains!
	if b.cfg.General.DomainsFile != "" {
		if _, err := os.Stat(b.cfg.General.DomainsFile); err == nil {
			args = append(args, "--hostlist="+b.cfg.General.DomainsFile)
		}
	}

	// Strategy specific args
	switch b.cfg.General.Strategy {
	case "strategy_d":
		args = append(args, b.cfg.NFQWS.StrategyDArgs...)
	case "strategy_c":
		fallthrough
	default:
		args = append(args, b.cfg.NFQWS.StrategyCArgs...)
	}

	return args
}

func (b *NFQWSBackend) supervisorLoop(ctx context.Context, binPath string) {
	consecutiveFailures := 0
	maxFailures := 5

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.mu.Lock()
		if !b.isRunning {
			b.mu.Unlock()
			return
		}

		args := b.buildArgs()
		cmd := exec.CommandContext(ctx, binPath, args...)
		b.cmd = cmd
		b.mu.Unlock()

		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		utils.Info("Starting nfqws engine: %s %s", binPath, args)
		startTime := time.Now()

		if err := cmd.Start(); err != nil {
			utils.Error("Failed to launch nfqws: %v", err)
			consecutiveFailures++
			if consecutiveFailures >= maxFailures {
				utils.Error("nfqws exceeded maximum crash limit (%d times). Stopping supervisor.", maxFailures)
				b.mu.Lock()
				b.isRunning = false
				b.mu.Unlock()
				return
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// Read output streams
		go func() {
			if stdoutPipe != nil {
				scanner := bufio.NewScanner(stdoutPipe)
				for scanner.Scan() {
					utils.Debug("[nfqws] %s", scanner.Text())
				}
			}
		}()

		go func() {
			if stderrPipe != nil {
				scanner := bufio.NewScanner(stderrPipe)
				for scanner.Scan() {
					utils.Debug("[nfqws err] %s", scanner.Text())
				}
			}
		}()

		err := cmd.Wait()

		// If process ran for more than 10 seconds, reset failure counter
		if time.Since(startTime) > 10*time.Second {
			consecutiveFailures = 0
		}

		b.mu.Lock()
		running := b.isRunning
		b.mu.Unlock()

		if !running {
			return
		}

		utils.Warn("nfqws process exited unexpectedly: %v. Restarting...", err)
		consecutiveFailures++
		if consecutiveFailures >= maxFailures {
			utils.Error("nfqws crashed %d times in rapid succession. Halting to avoid infinite loop.", maxFailures)
			b.mu.Lock()
			b.isRunning = false
			b.mu.Unlock()
			return
		}

		time.Sleep(1 * time.Second)
	}
}
