package wa

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Reconnection is deliberately slow. Reconnecting in a tight loop is one of the
// patterns that gets an unofficial WhatsApp number blocked, so a session that
// keeps failing backs off to five minutes and stays there.
const (
	minReconnectDelay = 5 * time.Second
	maxReconnectDelay = 5 * time.Minute
)

// Options are what a Client needs to do its work.
type Options struct {
	Container *sqlstore.Container
	AccountID uuid.UUID
	// DeviceJID is the number this account is already paired to, empty when it
	// has never been paired. It is what picks this session's device out of a
	// store now shared by every CS number.
	DeviceJID     string
	DB            *gorm.DB
	Publisher     *Publisher
	Logger        *zap.Logger
	Conversations *services.CSConversationService
	Messages      *services.CSMessageService
	Assignment    *services.CSAssignmentService
	MediaRoot     string
}

// Client is the one thing in TikMan that holds a WhatsApp session. It satisfies
// Sender, so the outbox drains through the same connection that receives.
type Client struct {
	wa        *whatsmeow.Client
	db        *gorm.DB
	accountID uuid.UUID
	publisher *Publisher
	logger    *zap.Logger
	inbound   *inboundHandler
	receipts  *receiptHandler
	presence  *presenceHandler
	// dropped carries "the socket went away" from an event handler to the
	// supervisor. It holds one slot because a second drop before the first is
	// answered says nothing new.
	dropped chan struct{}
	// paired carries "a device just linked" from the pairing event handler to
	// the supervisor. A session that starts — or becomes, after a logout —
	// unpaired has nothing for the supervisor to reconnect, and it waits here
	// instead of exiting: exiting would leave nothing running to recover the
	// very session this pairing is about to produce.
	paired chan struct{}
	// connected carries "this session is authenticated" to whoever has to wait
	// for it. Connect returns once the noise handshake is sent, which is well
	// before whatsmeow can serve a request, so anything that asks WhatsApp a
	// question at startup waits here instead. One slot, like the others: a
	// reconnect nobody is waiting on says nothing new.
	connected chan struct{}
	// ctx is the process lifetime, and is what the event handlers work under:
	// whatsmeow hands them no context of their own.
	ctx context.Context
}

// NewClient loads the stored WhatsApp session, or prepares an empty one for
// pairing when there is none.
func NewClient(ctx context.Context, opt Options) (*Client, error) {
	if err := opt.Container.Upgrade(ctx); err != nil {
		return nil, fmt.Errorf("prepare whatsapp session store: %w", err)
	}
	device, err := deviceFor(ctx, opt.Container, opt.DeviceJID)
	if err != nil {
		return nil, fmt.Errorf("load whatsapp session: %w", err)
	}

	wac := whatsmeow.NewClient(device, waLog.Noop)
	// whatsmeow's own reconnect starts immediately and widens by two seconds a
	// try, with no ceiling. This process backs off on its own terms instead.
	wac.EnableAutoReconnect = false

	client := &Client{
		wa:        wac,
		db:        opt.DB,
		accountID: opt.AccountID,
		publisher: opt.Publisher,
		logger:    opt.Logger,
		dropped:   make(chan struct{}, 1),
		paired:    make(chan struct{}, 1),
		connected: make(chan struct{}, 1),
		ctx:       ctx,
	}
	client.inbound = &inboundHandler{
		wa:            wac,
		accountID:     opt.AccountID,
		conversations: opt.Conversations,
		messages:      opt.Messages,
		assignment:    opt.Assignment,
		publisher:     opt.Publisher,
		media:         mediaStore{root: opt.MediaRoot},
		logger:        opt.Logger,
	}
	client.receipts = &receiptHandler{messages: opt.Messages, publisher: opt.Publisher}
	client.presence = &presenceHandler{
		accountID:     opt.AccountID,
		conversations: opt.Conversations,
		publisher:     opt.Publisher,
	}
	return client, nil
}

// NeedsPairing says whether this process still has to be shown a WhatsApp
// account before it can do anything.
func (c *Client) NeedsPairing() bool {
	return c.wa.Store.ID == nil
}

// Connected answers a channel that receives once the session is authenticated
// and can actually serve a request. It is what startup work has to wait on:
// Connect returns as soon as the handshake is sent.
func (c *Client) Connected() <-chan struct{} {
	return c.connected
}

// Connect opens the session and keeps it open for as long as ctx lives.
// Nothing here is fatal, paired or not: supervise waits for a pairing to
// succeed instead of exiting when the store has no device (see supervise),
// so there is always something alive to recover — reconnect() finding the
// network again, or an admin pairing later through the control channel this
// same process is about to start listening on. Crashing here over what is
// usually a boot-time network hiccup would tear that listener down before it
// ever started.
func (c *Client) Connect(ctx context.Context) {
	c.ctx = ctx
	// This is the context whatsmeow runs its own background work under — the
	// keepalive and handler loops among it — so shutting the process down stops
	// them too instead of leaving them on a dead socket.
	c.wa.BackgroundEventCtx = ctx
	c.wa.AddEventHandler(c.route)
	go c.supervise(ctx)

	if err := c.wa.Connect(); err != nil {
		// A network hiccup at boot must not become a restart loop: exiting here
		// would let the container manager retry faster, and harder, than the
		// backoff this process was given to protect the number.
		c.logger.Warn("Could not open the WhatsApp session at startup; retrying in the background",
			zap.Error(err))
		c.setStatus(ctx, models.WAAccountDisconnected)
		c.signalDropped()
	}
}

// Disconnect closes the session without giving up the pairing.
func (c *Client) Disconnect() {
	c.wa.Disconnect()
}

// Logout gives up the pairing. The next start needs pairing again.
func (c *Client) Logout(ctx context.Context) error {
	return c.wa.Logout(ctx)
}

func (c *Client) route(rawEvt any) {
	// One number's bad event must not silence the others. This process holds
	// every CS number now, and whatsmeow calls this handler on its own
	// goroutine — an unrecovered panic here would take the whole process, and
	// with it every other session, down over one malformed message.
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("Panic while handling a WhatsApp event",
				zap.String("account_id", c.accountID.String()),
				zap.Any("panic", r), zap.Stack("stack"))
		}
	}()

	switch evt := rawEvt.(type) {
	case *events.Message:
		if err := c.inbound.handle(c.ctx, evt); err != nil {
			c.logger.Error("Could not store incoming WhatsApp message",
				zap.String("wa_message_id", evt.Info.ID), zap.Error(err))
		}
	case *events.Receipt:
		if err := c.receipts.handle(c.ctx, evt); err != nil {
			c.logger.Error("Could not apply WhatsApp receipt", zap.Error(err))
		}
	case *events.ChatPresence:
		if err := c.presence.handle(c.ctx, evt); err != nil {
			c.logger.Warn("Could not announce that a customer is typing", zap.Error(err))
		}
	default:
		c.routeSession(rawEvt)
	}
}

// routeSession handles the events that say what the connection is doing,
// rather than what a customer sent.
func (c *Client) routeSession(rawEvt any) {
	switch evt := rawEvt.(type) {
	case *events.PairSuccess:
		// Fired for both QR and phone-number pairing once the phone approves —
		// the store now has a device, so the supervisor no longer needs to
		// wait for one.
		c.signalPaired()
	case *events.PairError:
		// The phone approved, but whatsmeow could not finish pairing locally.
		// Nothing else clears the "pairing" row the API wrote before this —
		// PairSuccess never fired, so signalPaired never will either.
		c.logger.Error("WhatsApp pairing failed after the phone approved", zap.Error(evt.Error))
		c.setStatus(c.ctx, models.WAAccountDisconnected)
	case *events.Connected:
		c.setStatus(c.ctx, models.WAAccountConnected)
		c.goOnline(c.ctx)
		c.signalConnected()
	case *events.Disconnected:
		c.setStatus(c.ctx, models.WAAccountDisconnected)
		c.signalDropped()
	case *events.StreamReplaced:
		// The one disconnect that must not be answered. Something else is now
		// holding this session; taking it back would have the two of us pulling
		// it from each other, which is the reconnect storm this whole backoff
		// exists to avoid. whatsmeow does not reconnect here either.
		c.logger.Error("Another client took over this WhatsApp session; staying disconnected")
		c.setStatus(c.ctx, models.WAAccountDisconnected)
	case *events.LoggedOut:
		c.logger.Warn("WhatsApp logged this number out. The process stays up but " +
			"will send and receive nothing until it is paired again")
		c.setStatus(c.ctx, models.WAAccountDisconnected)
		c.signalDropped()
	case *events.KeepAliveTimeout:
		// whatsmeow forces a reconnect here only when its own auto-reconnect is
		// on, and this process turned that off to own the backoff. Without this
		// a socket that stopped answering would stay open and silent.
		if time.Since(evt.LastSuccess) > whatsmeow.KeepAliveMaxFailTime {
			c.wa.ResetConnection()
		}
	case *events.TemporaryBan:
		c.logger.Error("WhatsApp temporarily banned this number", zap.Stringer("ban", evt))
		c.setStatus(c.ctx, models.WAAccountBanned)
	}
}

// setStatus records where the number stands and tells the browsers. Neither
// failure is worth stopping over: the connection itself is unaffected, and the
// status is read again on the next change.
func (c *Client) setStatus(ctx context.Context, status models.WAAccountStatus) {
	fields := map[string]any{"status": status}
	if status == models.WAAccountConnected {
		fields["last_connected_at"] = time.Now()
		if id := c.wa.Store.ID; id != nil {
			fields["jid"] = id.ToNonAD().String()
		}
	}

	err := c.db.Model(&models.WAAccount{}).Where("id = ?", c.accountID).Updates(fields).Error
	if err != nil {
		c.logger.Error("Could not record WhatsApp account status", zap.Error(err))
	}
	event := Event{
		Type:          EventAccountStatus,
		WAAccountID:   c.accountID.String(),
		AccountStatus: string(status),
	}
	if err := c.publisher.Publish(ctx, event); err != nil {
		c.logger.Warn("Could not announce WhatsApp account status", zap.Error(err))
	}
}

// goOnline tells WhatsApp this number is available.
//
// It is what makes the server push the customer's typing to us at all: chat
// state is only routed to a device the server believes somebody is looking at.
// whatsmeow also wants it called once per connection so the server has our
// pushname, without which the customer sees "-" where our name should be.
//
// The cost is that the number reads as online while the process runs, which for
// an inbox a team actually watches is closer to the truth than the alternative.
func (c *Client) goOnline(ctx context.Context) {
	if err := c.wa.SendPresence(ctx, types.PresenceAvailable); err != nil {
		c.logger.Warn("Could not mark this WhatsApp number available", zap.Error(err))
	}
}
