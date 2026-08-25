package connectivity

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHCommander executes OLT CLI commands over an authenticated SSH connection.
type SSHCommander struct {
	client  *ssh.Client
	timeout time.Duration
}

// NewSSHCommander connects to an OLT over SSH using password authentication.
func NewSSHCommander(host string, port int, username, password string, timeout time.Duration) (*SSHCommander, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	return &SSHCommander{client: client, timeout: timeout}, nil
}

// Close terminates the SSH connection.
func (c *SSHCommander) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// ExecuteCommand runs one command in a new SSH session.
func (c *SSHCommander) ExecuteCommand(ctx context.Context, cmd string) (*CommandResult, error) {
	start := time.Now()
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	resultCh := make(chan struct {
		output []byte
		err    error
	}, 1)
	go func() {
		output, runErr := session.CombinedOutput(cmd)
		resultCh <- struct {
			output []byte
			err    error
		}{output, runErr}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		commandResult := &CommandResult{Success: result.err == nil, Output: string(result.output), Duration: time.Since(start)}
		if result.err != nil {
			commandResult.Error = result.err.Error()
		}
		return commandResult, nil
	}
}

// BatchExecute runs commands sequentially, preserving one result per command.
func (c *SSHCommander) BatchExecute(ctx context.Context, cmds []string) ([]*CommandResult, error) {
	results := make([]*CommandResult, 0, len(cmds))
	for _, cmd := range cmds {
		result, err := c.ExecuteCommand(ctx, cmd)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}
