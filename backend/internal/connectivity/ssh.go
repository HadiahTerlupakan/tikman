package connectivity

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHTest performs SSH connection and authentication test
func SSHTest(ipAddress string, port int, username, password string, timeout time.Duration) error {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	address := net.JoinHostPort(ipAddress, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return fmt.Errorf("timeout after %s", timeout)
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return fmt.Errorf("authentication failed")
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	return nil
}
