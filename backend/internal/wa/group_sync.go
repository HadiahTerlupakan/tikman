// Mirroring the groups a number is in. The API holds no WhatsApp connection,
// so the list, the names and the members are copied into the database.
package wa

import (
	"context"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// selfAddresses is the paired number under both names WhatsApp gives it.
type selfAddresses struct {
	phone types.JID
	lid   types.JID
}

func (s selfAddresses) is(jids ...types.JID) bool {
	for _, jid := range jids {
		if jid.User == "" {
			continue
		}
		if jid.User == s.phone.User || jid.User == s.lid.User {
			return true
		}
	}
	return false
}

func (c *Client) self() selfAddresses {
	if c.wa.Store.ID == nil {
		return selfAddresses{}
	}
	return selfAddresses{phone: c.wa.Store.ID.ToNonAD(), lid: c.wa.Store.LID.ToNonAD()}
}

// SyncGroups re-reads every group the number is in. A failure leaves the
// mirror as it was, so a connection that is briefly down does not mark every
// group left.
func (c *Client) SyncGroups(ctx context.Context) (int, error) {
	infos, err := c.wa.GetJoinedGroups(ctx)
	if err != nil {
		return 0, fmt.Errorf("baca daftar grup: %w", err)
	}
	names := c.contactNames(ctx)
	self := c.self()
	snaps := make([]services.GroupSnapshot, 0, len(infos))
	for _, info := range infos {
		if info != nil {
			snaps = append(snaps, groupSnapshot(info, self, names))
		}
	}
	if err := c.groups.SyncAll(c.accountID, snaps); err != nil {
		return 0, err
	}
	c.publishGroup(ctx, "")
	return len(snaps), nil
}

// RefreshGroup re-reads one group, as after a change WhatsApp announced or an
// action this process carried out.
func (c *Client) RefreshGroup(ctx context.Context, jid types.JID) (*models.WAGroup, error) {
	info, err := c.wa.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("baca info grup: %w", err)
	}
	group, err := c.groups.Upsert(c.accountID, groupSnapshot(info, c.self(), c.contactNames(ctx)))
	if err != nil {
		return nil, err
	}
	c.publishGroup(ctx, group.ID.String())
	return group, nil
}

// contactNames is what the number's address book calls each person. A
// failure only costs the names.
func (c *Client) contactNames(ctx context.Context) map[types.JID]string {
	contacts, err := c.wa.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		c.logger.Warn("Could not read the contact names for the group members", zap.Error(err))
		return nil
	}
	names := make(map[types.JID]string, len(contacts))
	for jid, info := range contacts {
		if name := contactLabel(info); name != "" {
			names[jid.ToNonAD()] = name
		}
	}
	return names
}

func contactLabel(info types.ContactInfo) string {
	for _, name := range []string{info.FullName, info.FirstName, info.BusinessName, info.PushName} {
		if name != "" {
			return name
		}
	}
	return ""
}

// groupSnapshot reduces what whatsmeow answered to what the mirror keeps.
func groupSnapshot(info *types.GroupInfo, self selfAddresses, names map[types.JID]string) services.GroupSnapshot {
	snap := services.GroupSnapshot{
		JID:      info.JID.ToNonAD().String(),
		Name:     info.Name,
		Topic:    info.Topic,
		TopicID:  info.TopicID,
		OwnerJID: info.OwnerJID.ToNonAD().String(),
		Announce: info.IsAnnounce,
		Locked:   info.IsLocked,
	}
	if info.OwnerJID.IsEmpty() {
		snap.OwnerJID = ""
	}
	for _, p := range info.Participants {
		member := participantRow(p, self, names)
		if member.IsSelf && member.Role != models.GroupRoleMember {
			snap.IsAdmin = true
		}
		snap.Participants = append(snap.Participants, member)
	}
	return snap
}

func participantRow(p types.GroupParticipant, self selfAddresses, names map[types.JID]string) models.WAGroupParticipant {
	role := models.GroupRoleMember
	switch {
	case p.IsSuperAdmin:
		role = models.GroupRoleSuperAdmin
	case p.IsAdmin:
		role = models.GroupRoleAdmin
	}
	row := models.WAGroupParticipant{
		JID:    p.JID.ToNonAD().String(),
		Role:   role,
		IsSelf: self.is(p.JID, p.PhoneNumber, p.LID),
	}
	if !p.LID.IsEmpty() {
		row.LID = p.LID.ToNonAD().String()
	}
	if phone := phoneJID(p.JID, p.PhoneNumber); !phone.IsEmpty() {
		row.Phone = phone.User
	}
	for _, jid := range []types.JID{p.PhoneNumber, p.JID, p.LID} {
		if name, ok := names[jid.ToNonAD()]; ok && !jid.IsEmpty() {
			row.Name = name
			break
		}
	}
	if row.Name == "" {
		row.Name = p.DisplayName
	}
	return row
}

// handleGroupChange answers WhatsApp saying a group changed. Leaving is
// recorded straight away; anything else is re-read whole, which is simpler
// than applying each kind of change and cannot drift from what WhatsApp holds.
func (c *Client) handleGroupChange(evt *events.GroupInfo) {
	if c.self().is(evt.Leave...) {
		if err := c.groups.MarkLeft(c.accountID, evt.JID.ToNonAD().String()); err != nil {
			c.logger.Error("Could not record leaving a group", zap.Error(err))
		}
		c.publishGroup(c.ctx, "")
		return
	}
	c.refreshQuietly(evt.JID)
}

// storeJoinedGroup records a group the number was just added to, or created.
// The event carries the whole group, so nothing has to be asked for.
func (c *Client) storeJoinedGroup(evt *events.JoinedGroup) {
	group, err := c.groups.Upsert(c.accountID, groupSnapshot(&evt.GroupInfo, c.self(), c.contactNames(c.ctx)))
	if err != nil {
		c.logger.Error("Could not record a group this number joined", zap.Error(err))
		return
	}
	c.publishGroup(c.ctx, group.ID.String())
}

// refreshQuietly re-reads a group for an event handler, which has nobody to
// return an error to.
func (c *Client) refreshQuietly(jid types.JID) {
	if _, err := c.RefreshGroup(c.ctx, jid); err != nil {
		c.logger.Warn("Could not refresh a group", zap.String("group", jid.String()), zap.Error(err))
	}
}

func (c *Client) publishGroup(ctx context.Context, groupID string) {
	if err := c.publisher.Publish(ctx, Event{Type: EventGroup, GroupID: groupID}); err != nil {
		c.logger.Warn("Could not announce a group change", zap.Error(err))
	}
}
