package acp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// Terminal represents a running terminal session
type Terminal struct {
	ID       string
	Cmd      *exec.Cmd
	Output   bytes.Buffer
	mu       sync.Mutex
	done     chan struct{}
	exitCode *int
	signal   *string
}

// TerminalManager manages terminal instances
type TerminalManager struct {
	mu        sync.RWMutex
	terminals map[string]*Terminal
}

// NewTerminalManager creates a new terminal manager
func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		terminals: make(map[string]*Terminal),
	}
}

// CreateTerminal handles terminal creation requests
func (c *Client) CreateTerminal(ctx context.Context, p acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	// Create terminal ID
	terminalID := fmt.Sprintf("term_%d", len(c.terminals.terminals)+1)

	// Build command
	cmd := exec.CommandContext(ctx, p.Command, p.Args...)

	// Set working directory
	if p.Cwd != nil {
		cmd.Dir = *p.Cwd
	}

	// Set environment
	if len(p.Env) > 0 {
		env := cmd.Environ()
		for _, e := range p.Env {
			env = append(env, fmt.Sprintf("%s=%s", e.Name, e.Value))
		}
		cmd.Env = env
	}

	terminal := &Terminal{
		ID:   terminalID,
		Cmd:  cmd,
		done: make(chan struct{}),
	}

	// Capture output
	cmd.Stdout = &terminal.Output
	cmd.Stderr = &terminal.Output

	// Start the command
	if err := cmd.Start(); err != nil {
		return acpsdk.CreateTerminalResponse{}, err
	}

	// Store terminal
	c.terminals.mu.Lock()
	c.terminals.terminals[terminalID] = terminal
	c.terminals.mu.Unlock()

	// Wait for completion in background
	go func() {
		defer close(terminal.done)
		err := cmd.Wait()
		terminal.mu.Lock()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				terminal.exitCode = &code
			}
		} else {
			code := 0
			terminal.exitCode = &code
		}
		terminal.mu.Unlock()
	}()

	return acpsdk.CreateTerminalResponse{
		TerminalId: terminalID,
	}, nil
}

// KillTerminalCommand handles terminal kill requests
func (c *Client) KillTerminalCommand(ctx context.Context, p acpsdk.KillTerminalCommandRequest) (acpsdk.KillTerminalCommandResponse, error) {
	c.terminals.mu.RLock()
	terminal, ok := c.terminals.terminals[p.TerminalId]
	c.terminals.mu.RUnlock()

	if !ok {
		return acpsdk.KillTerminalCommandResponse{}, fmt.Errorf("terminal not found: %s", p.TerminalId)
	}

	if terminal.Cmd != nil && terminal.Cmd.Process != nil {
		if err := terminal.Cmd.Process.Kill(); err != nil {
			return acpsdk.KillTerminalCommandResponse{}, err
		}
		sig := "SIGKILL"
		terminal.mu.Lock()
		terminal.signal = &sig
		terminal.mu.Unlock()
	}

	return acpsdk.KillTerminalCommandResponse{}, nil
}

// TerminalOutput handles terminal output requests
func (c *Client) TerminalOutput(ctx context.Context, p acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	c.terminals.mu.RLock()
	terminal, ok := c.terminals.terminals[p.TerminalId]
	c.terminals.mu.RUnlock()

	if !ok {
		return acpsdk.TerminalOutputResponse{}, fmt.Errorf("terminal not found: %s", p.TerminalId)
	}

	terminal.mu.Lock()
	defer terminal.mu.Unlock()

	output := terminal.Output.String()
	truncated := false

	// Check if command has completed
	var exitStatus *acpsdk.TerminalExitStatus
	if terminal.exitCode != nil {
		exitStatus = &acpsdk.TerminalExitStatus{
			ExitCode: terminal.exitCode,
		}
	}

	return acpsdk.TerminalOutputResponse{
		Output:     output,
		Truncated:  truncated,
		ExitStatus: exitStatus,
	}, nil
}

// ReleaseTerminal handles terminal release requests
func (c *Client) ReleaseTerminal(ctx context.Context, p acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	c.terminals.mu.Lock()
	terminal, ok := c.terminals.terminals[p.TerminalId]
	if ok {
		// Kill if still running
		if terminal.Cmd != nil && terminal.Cmd.Process != nil {
			terminal.Cmd.Process.Kill()
		}
		delete(c.terminals.terminals, p.TerminalId)
	}
	c.terminals.mu.Unlock()

	if !ok {
		return acpsdk.ReleaseTerminalResponse{}, fmt.Errorf("terminal not found: %s", p.TerminalId)
	}

	return acpsdk.ReleaseTerminalResponse{}, nil
}

// WaitForTerminalExit handles terminal exit wait requests
func (c *Client) WaitForTerminalExit(ctx context.Context, p acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	c.terminals.mu.RLock()
	terminal, ok := c.terminals.terminals[p.TerminalId]
	c.terminals.mu.RUnlock()

	if !ok {
		return acpsdk.WaitForTerminalExitResponse{}, fmt.Errorf("terminal not found: %s", p.TerminalId)
	}

	// Wait for command to complete
	select {
	case <-terminal.done:
	case <-ctx.Done():
		return acpsdk.WaitForTerminalExitResponse{}, ctx.Err()
	}

	terminal.mu.Lock()
	defer terminal.mu.Unlock()

	return acpsdk.WaitForTerminalExitResponse{
		ExitCode: terminal.exitCode,
		Signal:   terminal.signal,
	}, nil
}