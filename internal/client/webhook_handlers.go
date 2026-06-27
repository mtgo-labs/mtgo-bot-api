package client

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mtgo-labs/mtgo/tg"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
	"github.com/mtgo-labs/mtgo-bot-api/internal/webhook"
)

func init() {
	Register("setwebhook", (*Client).setWebhook)
	Register("deletewebhook", (*Client).deleteWebhook)
	Register("getwebhookinfo", (*Client).getWebhookInfo)
}

// webhookMu protects deliverer creation/stop on the Client.
// The Client already has a mutex for connection state; we reuse a lightweight
// inline approach through the main mu.

// ensureDeliverer lazily creates the webhook deliverer after connection.
func (c *Client) ensureDeliverer() *webhook.Deliverer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deliverer == nil && c.store != nil {
		c.deliverer = webhook.NewDeliverer(c.botID, c.queueID(), c.params.TQueue, c.store)
	}
	return c.deliverer
}

// restoreWebhookLocked loads any persisted webhook config from the store and
// restarts the deliverer. Called from ensureConnected (caller already holds c.mu)
// so that webhooks survive server restarts, mirroring how the official Bot API
// reloads webhooks_db.binlog on boot.
func (c *Client) restoreWebhookLocked() {
	if c.store == nil {
		return
	}
	cfg, err := c.store.GetWebhookConfig(context.Background())
	if err != nil || cfg.URL == "" {
		return
	}
	c.applyAllowedUpdatesNormalized(cfg.AllowedUpdates)
	if c.deliverer == nil {
		c.deliverer = webhook.NewDeliverer(c.botID, c.queueID(), c.params.TQueue, c.store)
	}
	c.deliverer.SetResponseHandler(c.handleWebhookResponseMethod)
	c.deliverer.SetStatusHandler(c.handleWebhookDeliveryStatus)
	c.deliverer.Start(cfg, false)
	botlog.Info("client %s: restored webhook %s", c.botID, cfg.URL)
}

// setWebhook registers a webhook URL and starts delivering updates to it.
// Reference: Client.cpp process_set_webhook_query.
//
// Required: url (may be empty to delete). Optional: certificate, max_connections,
// allowed_updates, drop_pending_updates, secret_token.
func (c *Client) setWebhook(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}

	webhookURL := q.Arg("url")

	// Empty URL = delete webhook (Bot API semantics).
	if webhookURL == "" {
		return c.doDeleteWebhook(ctx, q.ArgBool("drop_pending_updates"))
	}

	maxConns := 40
	if v := q.Arg("max_connections"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, NewError(400, "Bad Request: parameter \"max_connections\" must be an integer")
		}
		maxLimit := 100
		if c.params.LocalMode {
			maxLimit = 100000
		}
		if n < 1 || n > maxLimit {
			return nil, NewError(400, fmt.Sprintf("Bad Request: max_connections must be between 1 and %d", maxLimit))
		}
		maxConns = n
	}

	allowedUpdates := c.currentAllowedUpdatesJSON()
	if q.HasArg("allowed_updates") {
		if _, normalized, ok := parseAllowedUpdates(q.Arg("allowed_updates")); ok {
			c.applyAllowedUpdatesNormalized(normalized)
			allowedUpdates = normalized
		}
	}

	cfg := storage.WebhookConfig{
		URL:            webhookURL,
		SecretToken:    q.Arg("secret_token"),
		IPAddress:      q.Arg("ip_address"),
		FixIP:          webhookFixIP(q),
		MaxConnections: maxConns,
		AllowedUpdates: allowedUpdates,
	}

	if webhookURL != "" {
		parsed, err := url.ParseRequestURI(webhookURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, NewError(400, "Bad Request: invalid webhook URL specified")
		}
		if !c.params.LocalMode && !webhookPortAllowed(parsed) {
			return nil, NewError(400, "Bad Request: bad webhook: Webhook can be set up only on ports 80, 88, 443 or 8443")
		}
		if !c.params.LocalMode && webhook.IsForbiddenHost(parsed.Hostname()) {
			return nil, NewError(400, "Bad Request: bad webhook: webhook address is in a forbidden range")
		}
		if len(cfg.SecretToken) > 256 {
			return nil, NewError(400, "Bad Request: secret token is too long")
		}
		for _, r := range cfg.SecretToken {
			if !(r == '-' || r == '_' || r == '.' || r == '~' ||
				(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				return nil, NewError(400, "Bad Request: secret token contains unallowed characters")
			}
		}
	}

	// Optional self-signed certificate (PEM). Telegram uses it as the trusted
	// root when delivering to an HTTPS webhook with a self-signed cert.
	if certFile, ok := q.File("certificate"); ok {
		data, err := os.ReadFile(certFile.TempPath)
		if err != nil {
			return nil, NewError(400, "Bad Request: failed to read certificate: "+err.Error())
		}
		if len(data) > 3<<20 {
			return nil, NewError(400, "Bad Request: certificate size is too big")
		}
		cfg.Certificate = data
	}

	now := time.Now()
	c.mu.Lock()
	if c.webhookSetBusy {
		c.mu.Unlock()
		return nil, &Error{
			Code:        429,
			Description: "Too Many Requests: retry after 1",
			Params:      &response.Parameters{RetryAfter: 1},
		}
	}
	if now.Before(c.nextSetWebhook) {
		c.mu.Unlock()
		return nil, &Error{
			Code:        429,
			Description: "Too Many Requests: retry after 1",
			Params:      &response.Parameters{RetryAfter: 1},
		}
	}
	c.webhookSetBusy = true
	c.nextSetWebhook = now.Add(time.Second)
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.webhookSetBusy = false
		c.mu.Unlock()
	}()

	if current, err := c.store.GetWebhookConfig(ctx); err == nil && sameWebhookConfig(current, cfg) && !q.ArgBool("drop_pending_updates") {
		return NewSuccess(true, "Webhook is already set"), nil
	}

	c.longPollMu.Lock()
	c.abortLongPollLocked(true)
	c.longPollMu.Unlock()

	if err := c.store.SetWebhookConfig(ctx, cfg); err != nil {
		return nil, NewError(500, "Internal Server Error: failed to save webhook config: "+err.Error())
	}

	d := c.ensureDeliverer()
	if d != nil {
		d.SetResponseHandler(c.handleWebhookResponseMethod)
		d.SetStatusHandler(c.handleWebhookDeliveryStatus)
		verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := d.Verify(verifyCtx, cfg)
		cancel()
		if err != nil {
			_ = c.store.DeleteWebhookConfig(ctx)
			return nil, NewError(400, "Bad Request: bad webhook: "+err.Error())
		}
		d.Start(cfg, q.ArgBool("drop_pending_updates"))
	}

	return NewSuccess(true, "Webhook was set"), nil
}

// deleteWebhook stops webhook delivery and clears the config.
// Reference: Client.cpp process_delete_webhook_query.
func (c *Client) deleteWebhook(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	return c.doDeleteWebhook(ctx, q.ArgBool("drop_pending_updates"))
}

// doDeleteWebhook is the shared delete path.
func (c *Client) doDeleteWebhook(ctx context.Context, dropPending bool) (any, error) {
	wasActive := false
	if c.store != nil {
		if cfg, err := c.store.GetWebhookConfig(ctx); err == nil && cfg.URL != "" {
			wasActive = true
		}
	}
	c.mu.Lock()
	if c.deliverer != nil {
		c.deliverer.Stop()
	}
	c.mu.Unlock()

	if err := c.store.DeleteWebhookConfig(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, NewError(500, "Internal Server Error: failed to delete webhook config: "+err.Error())
	}
	if dropPending && c.params.TQueue != nil {
		c.params.TQueue.Clear(ctx, c.queueID(), 0)
	}
	if !wasActive {
		return NewSuccess(true, "Webhook is already deleted"), nil
	}
	return NewSuccess(true, "Webhook was deleted"), nil
}

// getWebhookInfo returns the current webhook status.
// Reference: Client.cpp process_get_webhook_info_query.
func (c *Client) getWebhookInfo(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}

	cfg, err := c.store.GetWebhookConfig(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, NewError(500, "Internal Server Error: "+err.Error())
	}

	mask, _ := allowedUpdatesMask(cfg.AllowedUpdates)
	allowed := allowedUpdatesNames(mask, false)

	pending := 0
	if c.params.TQueue != nil {
		pending = c.params.TQueue.Size(c.queueID())
	}

	info := &apitypes.WebhookInfo{
		URL:                  cfg.URL,
		HasCustomCertificate: len(cfg.Certificate) > 0,
		PendingUpdateCount:   pending,
		LastErrorDate:        cfg.LastErrorDate,
		LastErrorMessage:     cfg.LastErrorMessage,
		IPAddress:            cfg.IPAddress,
		MaxConnections:       cfg.MaxConnections,
		AllowedUpdates:       allowed,
	}
	return info, nil
}

// webhookFixIP mirrors Client.cpp get_webhook_fix_ip_address: when fix_ip_address is
// present use its bool value, otherwise default to true when ip_address is set.
func webhookFixIP(q *server.Query) bool {
	if q.HasArg("fix_ip_address") {
		return q.ArgBool("fix_ip_address")
	}
	return q.Arg("ip_address") != ""
}

func webhookPortAllowed(u *url.URL) bool {
	port := u.Port()
	if port == "" {
		return true
	}
	switch port {
	case "80", "88", "443", "8443":
		return true
	default:
		return false
	}
}

func sameWebhookConfig(a, b storage.WebhookConfig) bool {
	return a.URL == b.URL &&
		a.SecretToken == b.SecretToken &&
		a.IPAddress == b.IPAddress &&
		a.FixIP == b.FixIP &&
		a.MaxConnections == b.MaxConnections &&
		a.AllowedUpdates == b.AllowedUpdates &&
		bytes.Equal(a.Certificate, b.Certificate)
}

func (c *Client) hasActiveWebhook() bool {
	c.mu.Lock()
	busy := c.webhookSetBusy
	deliverer := c.deliverer
	c.mu.Unlock()
	if busy {
		return true
	}
	if deliverer != nil && deliverer.IsActive() {
		return true
	}
	if c.store == nil {
		return false
	}
	cfg, err := c.store.GetWebhookConfig(context.Background())
	return err == nil && cfg.URL != ""
}

func (c *Client) handleWebhookResponseMethod(ctx context.Context, args map[string]string) {
	method := args["method"]
	if method == "" {
		return
	}
	q := server.NewQuery()
	q.Token = c.Token
	q.Method = method
	for k, v := range args {
		if k != "method" {
			q.Args[k] = v
		}
	}
	c.Dispatch(ctx, q)
}

const (
	minPendingUpdatesWarning = 200
	botUpdatesWarningDelay   = 30 * time.Second
)

func (c *Client) handleWebhookDeliveryStatus(ctx context.Context, success bool, errMessage string) {
	if c.rpc == nil || c.params.TQueue == nil {
		return
	}
	c.mu.Lock()
	if success {
		c.nextBotUpdatesWarningTime = time.Now().Add(botUpdatesWarningDelay)
		if !c.wasBotUpdatesWarning {
			c.mu.Unlock()
			return
		}
		c.wasBotUpdatesWarning = false
		c.mu.Unlock()
		c.sendBotUpdatesStatus(ctx, 0, "")
		return
	}
	pending := c.params.TQueue.Size(c.queueID())
	now := time.Now()
	if pending < minPendingUpdatesWarning || now.Before(c.nextBotUpdatesWarningTime) {
		c.mu.Unlock()
		return
	}
	c.nextBotUpdatesWarningTime = now.Add(botUpdatesWarningDelay)
	c.wasBotUpdatesWarning = true
	c.mu.Unlock()
	c.sendBotUpdatesStatus(ctx, pending, "Webhook error. "+errMessage)
}

func (c *Client) sendBotUpdatesStatus(ctx context.Context, pending int, message string) {
	if c.rpc == nil {
		return
	}
	if pending < 0 {
		pending = 0
	}
	if pending > int(^uint32(0)>>1) {
		pending = int(^uint32(0) >> 1)
	}
	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := c.rpc.HelpSetBotUpdatesStatus(statusCtx, &tg.HelpSetBotUpdatesStatusRequest{
		PendingUpdatesCount: int32(pending),
		Message:             message,
	}); err != nil {
		botlog.Warn("client %s: failed to set bot updates status: %v", c.botID, err)
	}
}
