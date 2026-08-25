package connectivity

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

func TestCommanderFactoryForOLTWithProtocolDecryptsPassword(t *testing.T) {
	server := startMockTelnetServer(t)
	host, portText, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatal(err)
	}
	const key = "12345678901234567890123456789012"
	encrypted, err := utils.Encrypt("admin123", key)
	if err != nil {
		t.Fatal(err)
	}

	factory := NewCommanderFactoryWithEncryption(5*time.Second, key)
	commander, err := factory.ForOLTWithProtocol(models.OLTModelZTEC300, host, models.OLTProtocolTelnet, port, "admin", encrypted)
	if err != nil {
		t.Fatalf("expected decrypted credentials to authenticate: %v", err)
	}
	defer closeCommanderForTest(commander)
}

func closeCommanderForTest(commander OLTCommander) {
	if closer, ok := commander.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}
