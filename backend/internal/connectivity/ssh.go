package connectivity

import (
	"fmt"
	"net"
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

	address := fmt.Sprintf("%s:%d", ipAddress, port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return fmt.Errorf("timeout after %s", timeout)
		}
		if err.Error() == "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain" {
			return fmt.Errorf("authentication failed")
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	return nil
}
