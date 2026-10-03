package wa

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// rescanPause spaces the saves a rescan makes. Hundreds of contact patches in
// a burst is the kind of pattern that gets an unofficial number flagged; at
// this pace three hundred subscribers take ten minutes.
const rescanPause = 2 * time.Second

// billedSubscriber is one subscriber a stored billing message names.
type billedSubscriber struct {
	phone types.JID
	lid   string
	name  string
}

// RescanBilledContacts saves every subscriber whose billing message is stored
// in this number's inbox, as if that message had just been sent. Only those
// with a thread are reachable: a billing message to anyone else was never
// stored, and lives only on the phone.
func (c *Client) RescanBilledContacts(ctx context.Context) {
	subscribers, err := billedSubscribers(c.db, c.accountID)
	if err != nil {
		c.logger.Error("Could not read the stored billing messages", zap.Error(err))
		return
	}
	c.logger.Info("Rescanning stored billing messages for contacts", zap.Int("subscribers", len(subscribers)))
	for _, sub := range subscribers {
		select {
		case <-ctx.Done():
			return
		case <-time.After(rescanPause):
		}
		c.contacts.saveBilled(ctx, sub.phone, sub.name, sub.lid)
	}
	c.logger.Info("Finished rescanning stored billing messages", zap.Int("subscribers", len(subscribers)))
}

// billedSubscribers answers each subscriber with a stored billing message on
// accountID, named by the newest one: a subscriber renamed in the billing app
// is saved under what it calls them now.
func billedSubscribers(db *gorm.DB, accountID uuid.UUID) ([]billedSubscriber, error) {
	// Named outright: GORM's naming reads CustomerJID as customer_j_id, and
	// the scan would leave it empty without a word.
	var rows []struct {
		CustomerJID   string `gorm:"column:customer_jid"`
		CustomerPhone string `gorm:"column:customer_phone"`
		Body          string `gorm:"column:body"`
	}
	// LIKE narrows the read to greetings before billedName decides; it cannot
	// be the whole test, since it does not know where the name ends.
	err := db.Table("cs_messages AS m").
		Select("c.customer_jid, c.customer_phone, m.body").
		Joins("JOIN cs_conversations AS c ON c.id = m.conversation_id").
		Where("c.wa_account_id = ? AND m.direction = ? AND m.body LIKE ?",
			accountID, models.MessageOut, "%Bapak/Ibu%").
		Order("m.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var subscribers []billedSubscriber
	for _, row := range rows {
		name := billedName(row.Body)
		if name == "" || seen[row.CustomerJID] {
			continue
		}
		seen[row.CustomerJID] = true
		jid, _ := types.ParseJID(row.CustomerJID)
		subscribers = append(subscribers, billedSubscriber{
			phone: threadPhone(jid, row.CustomerPhone),
			lid:   jid.User,
			name:  name,
		})
	}
	return subscribers, nil
}

// threadPhone answers a thread's phone-number address. A thread opened under a
// LID keeps the number in customer_phone when WhatsApp gave one, and the LID
// itself there when it did not.
func threadPhone(jid types.JID, customerPhone string) types.JID {
	if jid.Server == types.DefaultUserServer {
		return jid.ToNonAD()
	}
	if customerPhone == "" || customerPhone == jid.User {
		return types.EmptyJID
	}
	return types.NewJID(customerPhone, types.DefaultUserServer)
}
