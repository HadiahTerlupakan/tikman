package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WireGuardServer is the VPS side of the tunnel. Exactly one row exists. The
// service generates the keypair itself, so a private key never arrives from
// user input.
type WireGuardServer struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	InterfaceName string    `gorm:"type:varchar(15);not null;uniqueIndex;default:wg0"`
	ListenPort    int       `gorm:"not null;default:51820"`
	PrivateKey    string    `gorm:"type:text;not null"` // encrypted
	PublicKey     string    `gorm:"type:text;not null"`
	EndpointHost  string    `gorm:"type:varchar(255);not null"`
	TunnelSubnet  string    `gorm:"type:varchar(45);not null;default:10.88.0.0/24"`
	Address       string    `gorm:"type:varchar(45);not null;default:10.88.0.1"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s *WireGuardServer) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (s *WireGuardServer) TableName() string {
	return "wireguard_server"
}

// WireGuardPeer is one site. It carries no endpoint: sites sit behind NAT and
// are the side that initiates, so the server learns their address from the
// handshake.
type WireGuardPeer struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey"`
	SiteID              uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex"`
	Name                string         `gorm:"type:varchar(255);not null"`
	PublicKey           string         `gorm:"type:text;not null"`
	PrivateKey          string         `gorm:"type:text;not null"` // encrypted
	PresharedKey        string         `gorm:"type:text"`          // encrypted, optional
	TunnelAddress       string         `gorm:"type:varchar(45);not null;uniqueIndex"`
	AllowedIPs          datatypes.JSON `gorm:"column:allowed_ips;type:jsonb;not null"`
	PersistentKeepalive int            `gorm:"not null;default:25"`
	Enabled             bool           `gorm:"not null;default:true"`
	LastHandshakeAt     *time.Time
	RxBytes             int64 `gorm:"not null;default:0"`
	TxBytes             int64 `gorm:"not null;default:0"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (p *WireGuardPeer) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (p *WireGuardPeer) TableName() string {
	return "wireguard_peers"
}

// AllowedIPsList decodes the stored subnets. An unset column reads as an empty
// list rather than an error, because a peer row is written before its subnets
// are known only in tests.
func (p *WireGuardPeer) AllowedIPsList() ([]string, error) {
	if len(p.AllowedIPs) == 0 {
		return nil, nil
	}
	var list []string
	if err := json.Unmarshal(p.AllowedIPs, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (p *WireGuardPeer) SetAllowedIPs(list []string) error {
	encoded, err := json.Marshal(list)
	if err != nil {
		return err
	}
	p.AllowedIPs = encoded
	return nil
}
