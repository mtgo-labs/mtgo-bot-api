// Package webhook implements outgoing webhook delivery plus the
// setWebhook/deleteWebhook/getWebhookInfo lifecycle. Mirrors
// telegram-bot-api/WebhookActor.cpp.
package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// Deliverer runs a background goroutine that polls the TQueue for updates and
// POSTs them to the configured webhook URL. One Deliverer per bot.
type Deliverer struct {
	botID   string
	queueID tqueue.QueueID
	store   *storage.Store
	tq      *tqueue.TQueue
	client  *http.Client

	mu              sync.Mutex
	cfg             storage.WebhookConfig
	certPool        *x509.CertPool // built from cfg.Certificate; nil when no custom cert
	responseHandler func(context.Context, map[string]string)
	statusHandler   func(context.Context, bool, string)
	active          bool
	cancel          context.CancelFunc
	done            chan struct{}
	firstError410   time.Time
	lastSuccessTime time.Time
	ipCache         webhookIPCache
	activeDialGate  *dialGate
	pendingDialGate *dialGate

	// bypassSSRF disables SSRF IP filtering for tests (loopback httptest
	// servers). Never set in production.
	bypassSSRF bool
}

type webhookIPCache struct {
	host      string
	port      string
	ip        string
	expiresAt time.Time
}

type dialGate struct {
	every time.Duration
	burst int
	mu    sync.Mutex
	times []time.Time
}

func newDialGate(every time.Duration, burst int) *dialGate {
	return &dialGate{every: every, burst: burst}
}

func (g *dialGate) wait(ctx context.Context) error {
	for {
		delay := g.reserveDelay(time.Now())
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (g *dialGate) reserveDelay(now time.Time) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	cutoff := now.Add(-g.every)
	kept := g.times[:0]
	for _, ts := range g.times {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	g.times = kept
	if len(g.times) < g.burst {
		g.times = append(g.times, now)
		return 0
	}
	wait := g.times[0].Add(g.every).Sub(now)
	if wait < 0 {
		wait = 0
	}
	return wait
}

func (d *Deliverer) SetResponseHandler(h func(context.Context, map[string]string)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.responseHandler = h
}

func (d *Deliverer) SetStatusHandler(h func(context.Context, bool, string)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.statusHandler = h
}

// SetSSRFBypass disables SSRF IP filtering. Intended for tests that use
// httptest servers on loopback; never call this in production.
func (d *Deliverer) SetSSRFBypass(bypass bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bypassSSRF = bypass
}

type pendingUpdate struct {
	event     tqueue.Event
	delay     time.Duration
	wakeup    time.Time
	failCount int
	queueID   int64
	inFlight  bool
}

type deliverResult struct {
	eventID    tqueue.EventID
	queueID    int64
	forget     bool
	retryAfter int
	close      bool
}

// NewDeliverer creates a Deliverer (not yet started).
func NewDeliverer(botID string, qid tqueue.QueueID, tq *tqueue.TQueue, store *storage.Store) *Deliverer {
	return &Deliverer{
		botID:   botID,
		queueID: qid,
		store:   store,
		tq:      tq,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{DialContext: ssrfDialContext},
		},
		activeDialGate:  newDialGate(500*time.Millisecond, 10),
		pendingDialGate: newDialGate(2*time.Second, 1),
	}
}

// Start launches the delivery goroutine with the given config. If already
// running, the existing goroutine is stopped first. Drops pending updates if
// dropPending is true.
func (d *Deliverer) Start(cfg storage.WebhookConfig, dropPending bool) {
	d.mu.Lock()
	if d.active {
		cancel := d.cancel
		done := d.done
		d.mu.Unlock()
		cancel()
		<-done
		d.mu.Lock()
	}
	defer d.mu.Unlock()
	if dropPending {
		d.tq.Clear(context.Background(), d.queueID, 0)
	}
	d.cfg = cfg
	d.certPool = buildCertPool(cfg.Certificate)
	d.client = d.httpClient()
	d.firstError410 = time.Time{}
	d.lastSuccessTime = time.Now().Add(-2 * webhookIPCacheTime)
	d.ipCache = webhookIPCache{}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.done = make(chan struct{})
	d.active = true
	go d.run(ctx)
}

// buildCertPool parses a PEM-encoded certificate into a trust pool. Returns nil
// when the cert is empty or unparseable (delivery then uses the default roots).
func buildCertPool(cert []byte) *x509.CertPool {
	if len(cert) == 0 {
		return nil
	}
	pool := x509.NewCertPool()
	added := false
	for remaining := cert; len(remaining) > 0; {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		pool.AddCert(c)
		added = true
	}
	if !added {
		return nil
	}
	return pool
}

const (
	webhookIPCacheTime       = 30 * time.Minute
	webhookMaxResendTimeout  = 60 * time.Second
	webhookMaxResendJitter   = 60 * time.Second
	webhookDropTimeout       = 23 * time.Hour
	webhookActiveSuccessTime = 10 * time.Second
)

// httpClient builds an http.Client. When a self-signed certificate was uploaded it is
// used as the trusted root (mirroring telegram-bot-api). Dials pass through an IP
// cache and connection flood gates modeled after WebhookActor.
func (d *Deliverer) httpClient() *http.Client {
	transport := &http.Transport{}
	if d.certPool != nil {
		transport.TLSClientConfig = &tls.Config{RootCAs: d.certPool}
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if err := d.waitForDialSlot(ctx); err != nil {
			return nil, err
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
			port = "443"
		}
		dialHost, err := d.resolveDialHost(ctx, host, port)
		if err != nil {
			return nil, err
		}
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, net.JoinHostPort(dialHost, port))
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func (d *Deliverer) waitForDialSlot(ctx context.Context) error {
	d.mu.Lock()
	active := !d.lastSuccessTime.IsZero() && d.lastSuccessTime.Add(webhookActiveSuccessTime).After(time.Now())
	activeGate := d.activeDialGate
	pendingGate := d.pendingDialGate
	d.mu.Unlock()
	if active {
		return activeGate.wait(ctx)
	}
	return pendingGate.wait(ctx)
}

func (d *Deliverer) resolveDialHost(ctx context.Context, host, port string) (string, error) {
	d.mu.Lock()
	cfg := d.cfg
	d.mu.Unlock()
	return d.resolveDialHostForConfig(ctx, cfg, host, port)
}

// Stop halts the delivery goroutine.
func (d *Deliverer) Stop() {
	_ = d.StopContext(context.Background())
}

// StopContext halts the delivery goroutine, returning ctx.Err if the goroutine
// doesn't exit before the context is done.
func (d *Deliverer) StopContext(ctx context.Context) error {
	d.mu.Lock()
	if !d.active {
		d.mu.Unlock()
		return nil
	}
	cancel := d.cancel
	done := d.done
	d.mu.Unlock()
	cancel()
	select {
	case <-done:
		d.mu.Lock()
		d.active = false
		d.mu.Unlock()
		return nil
	case <-ctx.Done():
		go func() {
			<-done
			d.mu.Lock()
			d.active = false
			d.mu.Unlock()
		}()
		return ctx.Err()
	}
}

// IsActive reports whether the deliverer is currently running.
func (d *Deliverer) IsActive() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.active
}

// run is the main delivery loop. It polls the TQueue for new updates and
// POSTs them to the webhook URL. On success (2xx) the update is forgotten
// (confirmed). On failure, the error is recorded and the update is retried
// after a backoff delay.
func (d *Deliverer) run(ctx context.Context) {
	defer close(d.done)

	const (
		batchSize      = 100
		pollDelay      = 100 * time.Millisecond
		baseBackoff    = 1 * time.Second
		maxRetryAfter  = 3600
		fallbackQueue0 = int64(1 << 60)
	)

	var fromID tqueue.EventID
	nextQueueID := fallbackQueue0
	pending := make(map[tqueue.EventID]*pendingUpdate)
	queues := make(map[int64][]tqueue.EventID)
	inFlight := 0
	results := make(chan deliverResult, 1024)
	pollTimer := time.NewTimer(pollDelay)
	if !pollTimer.Stop() {
		select {
		case <-pollTimer.C:
		default:
		}
	}
	defer pollTimer.Stop()
	resetPollTimer := func() <-chan time.Time {
		if !pollTimer.Stop() {
			select {
			case <-pollTimer.C:
			default:
			}
		}
		pollTimer.Reset(pollDelay)
		return pollTimer.C
	}

	maxConnections := d.maxConnections()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		now := int32(time.Now().Unix())
		events, err := d.tq.Get(ctx, d.queueID, fromID, false, now, batchSize)
		if err != nil {
			botlog.Error("webhook[%s]: queue get error: %v", d.botID, err)
			time.Sleep(pollDelay)
			continue
		}
		loadNow := time.Now()
		for _, ev := range events {
			if _, exists := pending[ev.ID]; exists {
				continue
			}
			qid := ev.Extra
			if qid == 0 {
				qid = nextQueueID
				nextQueueID++
			}
			pending[ev.ID] = &pendingUpdate{
				event:   ev,
				delay:   baseBackoff,
				wakeup:  loadNow,
				queueID: qid,
			}
			queues[qid] = append(queues[qid], ev.ID)
			if next, err := ev.ID.Next(); err == nil && next > fromID {
				fromID = next
			}
		}

		for inFlight < maxConnections {
			nowTime := time.Now()
			var chosen *pendingUpdate
			for _, ids := range queues {
				if len(ids) == 0 {
					continue
				}
				up := pending[ids[0]]
				if up == nil || up.inFlight || up.wakeup.After(nowTime) {
					continue
				}
				chosen = up
				break
			}
			if chosen == nil {
				break
			}
			chosen.inFlight = true
			inFlight++
			go func(up *pendingUpdate) {
				forget, retryAfter, closeWebhook := d.deliver(ctx, up.event)
				results <- deliverResult{
					eventID:    up.event.ID,
					queueID:    up.queueID,
					forget:     forget,
					retryAfter: retryAfter,
					close:      closeWebhook,
				}
			}(chosen)
		}

		if len(pending) == 0 && len(events) == 0 {
			select {
			case <-ctx.Done():
				return
			case res := <-results:
				_ = res
			case <-resetPollTimer():
			}
			continue
		}

		select {
		case <-ctx.Done():
			return
		case res := <-results:
			inFlight--
			up := pending[res.eventID]
			if up == nil {
				continue
			}
			if res.close {
				if d.store != nil {
					_ = d.store.DeleteWebhookConfig(ctx)
				}
				d.mu.Lock()
				d.active = false
				d.mu.Unlock()
				return
			}
			up.inFlight = false
			if res.forget {
				d.tq.Forget(ctx, d.queueID, res.eventID)
				delete(pending, res.eventID)
				ids := queues[res.queueID]
				if len(ids) > 0 && ids[0] == res.eventID {
					ids = ids[1:]
				}
				if len(ids) == 0 {
					delete(queues, res.queueID)
				} else {
					queues[res.queueID] = ids
				}
				continue
			}
			wait := up.delay
			if res.retryAfter > 0 {
				if res.retryAfter > maxRetryAfter {
					res.retryAfter = maxRetryAfter
				}
				wait = time.Duration(res.retryAfter) * time.Second
			} else if up.failCount > 0 {
				up.delay *= 2
				maxBackoff := webhookMaxResendTimeout + time.Duration(rand.Int63n(int64(webhookMaxResendJitter)))
				if up.delay > maxBackoff {
					up.delay = maxBackoff
				}
				wait = up.delay
			}
			up.failCount++
			if up.event.ExpiresAt != 0 && time.Now().Add(wait).Unix() > int64(up.event.ExpiresAt) {
				d.tq.Forget(ctx, d.queueID, res.eventID)
				delete(pending, res.eventID)
				ids := queues[res.queueID]
				if len(ids) > 0 && ids[0] == res.eventID {
					ids = ids[1:]
				}
				if len(ids) == 0 {
					delete(queues, res.queueID)
				} else {
					queues[res.queueID] = ids
				}
				continue
			}
			up.wakeup = time.Now().Add(wait)
		case <-resetPollTimer():
		}
	}
}

func (d *Deliverer) maxConnections() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cfg.MaxConnections > 0 {
		return d.cfg.MaxConnections
	}
	return 40
}

func (d *Deliverer) Verify(ctx context.Context, cfg storage.WebhookConfig) error {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("host is empty")
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	if err := d.waitForDialSlot(ctx); err != nil {
		return err
	}
	addrHost, err := d.resolveDialHostForConfig(ctx, cfg, host, port)
	if err != nil {
		return err
	}
	var dialer net.Dialer
	rawConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addrHost, port))
	if err != nil {
		return err
	}
	defer func() { _ = rawConn.Close() }()
	if u.Scheme != "https" {
		return nil
	}
	tlsCfg := &tls.Config{ServerName: host}
	if pool := buildCertPool(cfg.Certificate); pool != nil {
		tlsCfg.RootCAs = pool
	}
	tlsConn := tls.Client(rawConn, tlsCfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return err
	}
	return nil
}

func (d *Deliverer) resolveDialHostForConfig(ctx context.Context, cfg storage.WebhookConfig, host, port string) (string, error) {
	d.mu.Lock()
	bypassSSRF := d.bypassSSRF
	d.mu.Unlock()

	// Fixed IP path: operator-configured IP (still subject to SSRF filter).
	if cfg.FixIP && cfg.IPAddress != "" {
		if !bypassSSRF {
			if ip, err := netip.ParseAddr(cfg.IPAddress); err == nil && isForbiddenIP(ip) {
				return "", fmt.Errorf("SSRF: forbidden IP %s for host %q", cfg.IPAddress, host)
			}
		}
		return cfg.IPAddress, nil
	}

	// Cache hit — the cached IP was validated at resolution time.
	d.mu.Lock()
	cache := d.ipCache
	now := time.Now()
	if cache.host == host && cache.port == port && cache.ip != "" && now.Before(cache.expiresAt) {
		d.mu.Unlock()
		return cache.ip, nil
	}
	d.mu.Unlock()

	// PreferGo resolver so we control DNS resolution (defeats system-level
	// resolver quirks that could bypass our SSRF filter).
	resolver := &net.Resolver{PreferGo: true}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP address found for %s", host)
	}

	// SSRF: reject if ANY resolved IP is forbidden. Checking all addresses
	// defeats DNS-rebinding where an attacker returns a public IP alongside
	// an internal one and relies on the client picking the internal one.
	if !bypassSSRF {
		for _, a := range ips {
			ip, err := netip.ParseAddr(a.IP.String())
			if err != nil || isForbiddenIP(ip) {
				return "", fmt.Errorf("SSRF: forbidden IP %s for host %q", a.IP, host)
			}
		}
	}

	// Cache and return the first resolved IP. Connecting to this specific IP
	// (rather than re-resolving in the Dialer) prevents rebinding between
	// validation and connection.
	ip := ips[0].IP.String()
	d.mu.Lock()
	d.ipCache = webhookIPCache{host: host, port: port, ip: ip, expiresAt: time.Now().Add(webhookIPCacheTime)}
	d.mu.Unlock()
	return ip, nil
}

// deliver POSTs a single update to the webhook URL. Returns (forget, retryAfter, closeWebhook).
// forget=true means the event should be forgotten (2xx success);
// the caller drops it from the queue and advances. forget=false means delivery
// failed; the caller should retry after backoff (or retryAfter seconds). closeWebhook
// mirrors WebhookActor's sustained-410 close path.
func (d *Deliverer) deliver(ctx context.Context, ev tqueue.Event) (bool, int, bool) {
	d.mu.Lock()
	cfg := d.cfg
	responseHandler := d.responseHandler
	statusHandler := d.statusHandler
	d.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(ev.Data))
	if err != nil {
		d.recordError(fmt.Sprintf("failed to create request: %v", err))
		return false, 0, false
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.SecretToken != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", cfg.SecretToken)
	}
	if u, err := url.Parse(cfg.URL); err == nil && u.User != nil {
		if password, ok := u.User.Password(); ok {
			req.SetBasicAuth(u.User.Username(), password)
		} else {
			req.SetBasicAuth(u.User.Username(), "")
		}
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.recordError(fmt.Sprintf("failed to deliver update: %v", err))
		return false, 0, false
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		d.reset410()
		if statusHandler != nil {
			go statusHandler(context.Background(), true, "")
		}
		if responseHandler != nil {
			if args := parseWebhookResponseMethod(body); len(args) > 0 {
				go responseHandler(context.Background(), args)
			}
		}
		return true, 0, false
	case resp.StatusCode == 410:
		msg := fmt.Sprintf("Wrong response from the webhook: %d %s", resp.StatusCode, resp.Status)
		d.recordError(msg)
		if statusHandler != nil {
			go statusHandler(context.Background(), false, msg)
		}
		return false, 0, d.mark410()
	case resp.StatusCode == 429:
		d.reset410()
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		msg := fmt.Sprintf("Wrong response from the webhook: %d %s", resp.StatusCode, resp.Status)
		d.recordError(msg)
		if statusHandler != nil {
			go statusHandler(context.Background(), false, msg)
		}
		return false, retryAfter, false
	default:
		d.reset410()
		msg := fmt.Sprintf("Wrong response from the webhook: %d %s", resp.StatusCode, resp.Status)
		d.recordError(msg)
		if statusHandler != nil {
			go statusHandler(context.Background(), false, msg)
		}
		return false, 0, false
	}
}

func (d *Deliverer) mark410() bool {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.firstError410.IsZero() {
		d.firstError410 = now
		return false
	}
	return now.After(d.firstError410.Add(webhookDropTimeout))
}

func (d *Deliverer) reset410() {
	d.mu.Lock()
	d.firstError410 = time.Time{}
	d.mu.Unlock()
}

func parseWebhookResponseMethod(body []byte) map[string]string {
	if len(body) == 0 {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}
	var method string
	if raw := obj["method"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &method)
	}
	if method == "" {
		return nil
	}
	methodLower := strings.ToLower(method)
	if methodLower == "deletewebhook" || methodLower == "setwebhook" ||
		methodLower == "close" || methodLower == "logout" ||
		strings.HasPrefix(methodLower, "get") {
		return nil
	}
	args := make(map[string]string, len(obj))
	args["method"] = methodLower
	for k, raw := range obj {
		if k == "method" {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			args[k] = s
		} else {
			args[k] = string(raw)
		}
	}
	return args
}

// recordError stores the last error in the per-bot store.
func (d *Deliverer) recordError(msg string) {
	now := time.Now().Unix()
	botlog.Error("webhook[%s]: %s", d.botID, msg)
	if d.store != nil {
		if err := d.store.SetWebhookError(context.Background(), now, msg); err != nil {
			botlog.Error("webhook[%s]: failed to record error: %v", d.botID, err)
		}
	}
}

// parseRetryAfter parses the Retry-After header (seconds).
func parseRetryAfter(val string) int {
	if val == "" {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 0
	}
	if n > 3600 {
		return 3600
	}
	return n
}
