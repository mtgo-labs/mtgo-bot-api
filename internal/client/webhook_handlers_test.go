package client

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

func newQuery(args map[string]string) *server.Query {
	q := server.NewQuery()
	for k, v := range args {
		q.Args[k] = v
	}
	return q
}

// TestWebhookFixIP mirrors Client.cpp get_webhook_fix_ip_address: fix_ip_address, if
// present, wins; otherwise it defaults to whether ip_address is set.
func TestWebhookFixIP(t *testing.T) {
	cases := []struct {
		name string
		args map[string]string
		want bool
	}{
		{"explicit true", map[string]string{"fix_ip_address": "true"}, true},
		{"explicit false", map[string]string{"fix_ip_address": "false"}, false},
		{"default with ip", map[string]string{"ip_address": "1.2.3.4"}, true},
		{"default no ip", map[string]string{}, false},
		{"explicit true overrides empty ip", map[string]string{"fix_ip_address": "true"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := webhookFixIP(newQuery(tc.args)); got != tc.want {
				t.Errorf("webhookFixIP(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestSetWebhookReturnsDescription(t *testing.T) {
	c := newWebhookTestClient(t, Params{LocalMode: true, TQueue: tqueue.New()})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	q := newQuery(map[string]string{"url": srv.URL})
	attachServerCert(t, q, srv)
	got, err := c.setWebhook(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	success, ok := got.(Success)
	if !ok {
		t.Fatalf("result type = %T, want Success", got)
	}
	if success.Description != "Webhook was set" {
		t.Fatalf("description = %q, want Webhook was set", success.Description)
	}
}

func TestSetWebhookAlreadySetDescription(t *testing.T) {
	c := newWebhookTestClient(t, Params{LocalMode: true, TQueue: tqueue.New()})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	q := newQuery(map[string]string{"url": srv.URL})
	attachServerCert(t, q, srv)
	if _, err := c.setWebhook(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	c.nextSetWebhook = time.Time{}
	got, err := c.setWebhook(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	success := got.(Success)
	if success.Description != "Webhook is already set" {
		t.Fatalf("description = %q", success.Description)
	}
}

func TestSetWebhookNormalizesAndPersistsAllowedUpdates(t *testing.T) {
	c := newWebhookTestClient(t, Params{LocalMode: true, TQueue: tqueue.New()})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	q := newQuery(map[string]string{
		"url":             srv.URL,
		"allowed_updates": `["POLL","message","unknown","poll"]`,
	})
	attachServerCert(t, q, srv)
	if _, err := c.setWebhook(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	cfg, err := c.store.GetWebhookConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowedUpdates != `["message","poll"]` {
		t.Fatalf("AllowedUpdates = %q", cfg.AllowedUpdates)
	}
	if !c.allowedUpdates["message"] || !c.allowedUpdates["poll"] || c.allowedUpdates["callback_query"] {
		t.Fatalf("allowedUpdates = %+v", c.allowedUpdates)
	}
}

func TestRestoreWebhookRestoresAllowedUpdates(t *testing.T) {
	c := newWebhookTestClient(t, Params{TQueue: tqueue.New()})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := c.store.SetWebhookConfig(context.Background(), storage.WebhookConfig{
		URL:            srv.URL,
		Certificate:    certPEM,
		AllowedUpdates: `["message","poll"]`,
	}); err != nil {
		t.Fatal(err)
	}
	c.restoreWebhookLocked()
	if c.deliverer != nil {
		defer c.deliverer.Stop()
	}
	if !c.allowedUpdates["message"] || !c.allowedUpdates["poll"] || c.allowedUpdates["callback_query"] {
		t.Fatalf("allowedUpdates = %+v", c.allowedUpdates)
	}
}

func TestDeleteWebhookReturnsDescription(t *testing.T) {
	c := newWebhookTestClient(t, Params{TQueue: tqueue.New()})
	if err := c.store.SetWebhookConfig(context.Background(), storage.WebhookConfig{URL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	got, err := c.doDeleteWebhook(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	success, ok := got.(Success)
	if !ok {
		t.Fatalf("result type = %T, want Success", got)
	}
	if success.Description != "Webhook was deleted" {
		t.Fatalf("description = %q, want Webhook was deleted", success.Description)
	}
}

func TestDeleteWebhookAlreadyDeletedDescription(t *testing.T) {
	c := newWebhookTestClient(t, Params{TQueue: tqueue.New()})
	got, err := c.doDeleteWebhook(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	success := got.(Success)
	if success.Description != "Webhook is already deleted" {
		t.Fatalf("description = %q", success.Description)
	}
}

func TestSetWebhookThrottle(t *testing.T) {
	c := newWebhookTestClient(t, Params{LocalMode: true, TQueue: tqueue.New()})
	c.nextSetWebhook = time.Now().Add(time.Second)
	_, err := c.setWebhook(context.Background(), newQuery(map[string]string{"url": "https://example.com"}))
	if err == nil || !strings.Contains(err.Error(), "retry after 1") {
		t.Fatalf("err = %v, want retry after 1", err)
	}
}

func TestSetWebhookValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args map[string]string
		want string
	}{
		{"bad url", map[string]string{"url": "http://example.com"}, "invalid webhook URL specified"},
		{"bad secret len", map[string]string{"url": "https://example.com", "secret_token": strings.Repeat("a", 257)}, "secret token is too long"},
		{"bad secret charset", map[string]string{"url": "https://example.com", "secret_token": "bad!"}, "secret token contains unallowed characters"},
		{"bad max", map[string]string{"url": "https://example.com", "max_connections": "0"}, "max_connections must be between 1 and 100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newWebhookTestClient(t, Params{})
			_, err := c.setWebhook(context.Background(), newQuery(tt.args))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestWebhookPortAllowed(t *testing.T) {
	for _, rawURL := range []string{"https://example.com", "https://example.com:80", "https://example.com:88", "https://example.com:443", "https://example.com:8443"} {
		u, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		if !webhookPortAllowed(u) {
			t.Fatalf("webhookPortAllowed(%s) = false", rawURL)
		}
	}
	u, err := url.Parse("https://example.com:9443")
	if err != nil {
		t.Fatal(err)
	}
	if webhookPortAllowed(u) {
		t.Fatal("webhookPortAllowed allowed forbidden port 9443")
	}
}

func TestSameWebhookConfigComparesCertificateContent(t *testing.T) {
	a := storage.WebhookConfig{URL: "https://example.com", Certificate: []byte("abc")}
	b := storage.WebhookConfig{URL: "https://example.com", Certificate: []byte("xyz")}
	if sameWebhookConfig(a, b) {
		t.Fatal("sameWebhookConfig returned true for different certificate bytes")
	}
}

func TestHandleWebhookDeliveryStatusReportsBacklog(t *testing.T) {
	r := &recorder{}
	tq := tqueue.New()
	c := newWebhookTestClient(t, Params{TQueue: tq})
	c.rpc = tg.NewRPCClient(r)
	exp := int32(time.Now().Add(time.Hour).Unix())
	for i := 0; i < minPendingUpdatesWarning; i++ {
		if _, err := tq.Push(context.Background(), c.queueID(), []byte(`{"message":{"message_id":1}}`), exp, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	c.handleWebhookDeliveryStatus(context.Background(), false, "Wrong response from the webhook: 500 Internal Server Error")
	req, ok := wantReq(t, r).(*tg.HelpSetBotUpdatesStatusRequest)
	if !ok {
		t.Fatalf("captured %T, want HelpSetBotUpdatesStatusRequest", r.last())
	}
	if req.PendingUpdatesCount != minPendingUpdatesWarning {
		t.Fatalf("pending = %d", req.PendingUpdatesCount)
	}
	if !strings.Contains(req.Message, "Webhook error. Wrong response from the webhook") {
		t.Fatalf("message = %q", req.Message)
	}

	c.handleWebhookDeliveryStatus(context.Background(), true, "")
	req, ok = wantReq(t, r).(*tg.HelpSetBotUpdatesStatusRequest)
	if !ok {
		t.Fatalf("captured %T, want HelpSetBotUpdatesStatusRequest", r.last())
	}
	if req.PendingUpdatesCount != 0 || req.Message != "" {
		t.Fatalf("clear request = %+v", req)
	}
}

func TestGetUpdatesRejectsPersistedWebhook(t *testing.T) {
	c := newWebhookTestClient(t, Params{TQueue: tqueue.New()})
	if err := c.store.SetWebhookConfig(context.Background(), storage.WebhookConfig{URL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	_, err := c.getUpdates(context.Background(), newQuery(nil))
	if err == nil || !strings.Contains(err.Error(), "webhook is active") {
		t.Fatalf("err = %v, want webhook conflict", err)
	}
}

func newWebhookTestClient(t *testing.T, params Params) *Client {
	t.Helper()
	store, err := storage.Open(t.TempDir(), "123")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	c := NewClient(params, "123:tok")
	c.store = store
	c.ready = true
	c.ensureDeliverer().SetSSRFBypass(true) // tests use httptest (loopback)
	return c
}

func attachServerCert(t *testing.T, q *server.Query, srv *httptest.Server) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cert.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	q.Files["certificate"] = server.File{
		FieldName: "certificate",
		FileName:  "cert.pem",
		TempPath:  path,
		Size:      int64(len(pemBytes)),
	}
}
