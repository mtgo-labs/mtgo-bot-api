package webhook

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// newTestDeliverer builds a Deliverer pointed at the given server URL with a
// nil store (recordError is a no-op when store is nil).
func newTestDeliverer(t *testing.T, url string) *Deliverer {
	t.Helper()
	d := NewDeliverer("test", tqueue.QueueID(1), tqueue.New(), nil)
	d.cfg = storage.WebhookConfig{URL: url}
	d.bypassSSRF = true // tests use httptest (loopback)
	d.client = &http.Client{}
	return d
}

func TestDeliver_StatusSwitch(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		retryAfter string // header value
		wantForget bool
		wantRetry  int
		wantClose  bool
	}{
		{name: "200 OK", status: 200, wantForget: true},
		{name: "204 No Content", status: 204, wantForget: true},
		{name: "400 Bad Request retries", status: 400},
		{name: "410 Gone retries before drop timeout", status: 410},
		{name: "429 with Retry-After", status: 429, retryAfter: "30", wantRetry: 30},
		{name: "429 with huge Retry-After clamps", status: 429, retryAfter: "99999", wantRetry: 3600},
		{name: "429 without Retry-After", status: 429},
		{name: "429 with invalid Retry-After", status: 429, retryAfter: "abc"},
		{name: "500 Internal Server Error retries", status: 500},
		{name: "502 Bad Gateway retries", status: 502},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			d := newTestDeliverer(t, srv.URL)
			ev := tqueue.Event{ID: tqueue.EventID(1), Data: []byte(`{}`)}
			forget, retry, closeWebhook := d.deliver(context.Background(), ev)

			if forget != tc.wantForget {
				t.Errorf("forget = %v, want %v", forget, tc.wantForget)
			}
			if retry != tc.wantRetry {
				t.Errorf("retryAfter = %v, want %v", retry, tc.wantRetry)
			}
			if closeWebhook != tc.wantClose {
				t.Errorf("closeWebhook = %v, want %v", closeWebhook, tc.wantClose)
			}
		})
	}
}

func TestDeliver_SecretTokenHeader(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	d := newTestDeliverer(t, srv.URL)
	d.cfg.SecretToken = "secret123"
	ev := tqueue.Event{ID: tqueue.EventID(1), Data: []byte(`{}`)}
	forget, _, _ := d.deliver(context.Background(), ev)

	if !forget {
		t.Fatal("expected forget=true for 200")
	}
	if gotToken != "secret123" {
		t.Errorf("secret token header = %q, want %q", gotToken, "secret123")
	}
}

func TestDeliver_BasicAuthFromURLUserinfo(t *testing.T) {
	var user, pass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword("alice", "secret")
	d := newTestDeliverer(t, parsed.String())
	ev := tqueue.Event{ID: tqueue.EventID(1), Data: []byte(`{}`)}
	forget, _, _ := d.deliver(context.Background(), ev)

	if !forget {
		t.Fatal("expected forget=true for 200")
	}
	if user != "alice" || pass != "secret" {
		t.Fatalf("basic auth = %q/%q, want alice/secret", user, pass)
	}
}

func TestDeliver_ParsesWebhookResponseMethod(t *testing.T) {
	called := make(chan map[string]string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"method":"sendMessage","chat_id":42,"text":"hi"}`))
	}))
	defer srv.Close()

	d := newTestDeliverer(t, srv.URL)
	d.SetResponseHandler(func(_ context.Context, args map[string]string) {
		called <- args
	})
	ev := tqueue.Event{ID: tqueue.EventID(1), Data: []byte(`{}`)}
	forget, _, _ := d.deliver(context.Background(), ev)
	if !forget {
		t.Fatal("expected forget=true")
	}
	select {
	case args := <-called:
		if args["method"] != "sendmessage" || args["chat_id"] != "42" || args["text"] != "hi" {
			t.Fatalf("args = %+v", args)
		}
	case <-time.After(time.Second):
		t.Fatal("response method was not dispatched")
	}
}

func TestDeliver_ClosesWebhookAfterSustained410(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	d := newTestDeliverer(t, srv.URL)
	d.firstError410 = time.Now().Add(-24 * time.Hour)
	ev := tqueue.Event{ID: tqueue.EventID(1), Data: []byte(`{}`)}
	forget, retry, closeWebhook := d.deliver(context.Background(), ev)
	if forget || retry != 0 || !closeWebhook {
		t.Fatalf("deliver = forget:%v retry:%d close:%v, want retry with close", forget, retry, closeWebhook)
	}
}

func TestVerifyChecksTLSReadinessWithoutGET(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "GET should not be required for verification", http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	d := newTestDeliverer(t, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := d.Verify(ctx, storage.WebhookConfig{URL: srv.URL, Certificate: certPEM}); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
}

func TestDialGateBurstAndDelay(t *testing.T) {
	gate := newDialGate(50*time.Millisecond, 2)
	start := time.Now()
	if err := gate.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gate.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gate.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 45*time.Millisecond {
		t.Fatalf("third dial was not delayed enough: %v", elapsed)
	}
}

func TestResolveDialHostUsesCacheAndExpiry(t *testing.T) {
	d := newTestDeliverer(t, "https://example.com")
	d.ipCache = webhookIPCache{host: "example.com", port: "443", ip: "127.0.0.1", expiresAt: time.Now().Add(time.Hour)}
	got, err := d.resolveDialHost(context.Background(), "example.com", "443")
	if err != nil {
		t.Fatal(err)
	}
	if got != "127.0.0.1" {
		t.Fatalf("cached IP = %q", got)
	}
	d.ipCache.expiresAt = time.Now().Add(-time.Second)
	got, err = d.resolveDialHost(context.Background(), "localhost", "443")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected resolved localhost IP")
	}
}

func TestStopContextHonorsDeadline(t *testing.T) {
	d := NewDeliverer("bot", 1, tqueue.New(), nil)
	d.mu.Lock()
	d.active = true
	d.cancel = func() {}
	d.done = make(chan struct{})
	d.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.StopContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StopContext err = %v, want context.Canceled", err)
	}
	if !d.IsActive() {
		t.Fatal("deliverer should remain active until done closes")
	}
	close(d.done)
	time.Sleep(10 * time.Millisecond)
	if d.IsActive() {
		t.Fatal("deliverer should become inactive after done closes")
	}
}

func TestHTTPClientFixIPDialsConfiguredAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	d := NewDeliverer("test", 1, tqueue.New(), nil)
	d.bypassSSRF = true // tests use loopback FixIP
	d.cfg = storage.WebhookConfig{
		URL:       "http://example.invalid:" + port,
		FixIP:     true,
		IPAddress: "127.0.0.1",
	}
	d.client = d.httpClient()
	forget, _, _ := d.deliver(context.Background(), tqueue.Event{ID: 1, Data: []byte(`{}`)})
	if !forget {
		t.Fatal("expected delivery through fixed IP to succeed")
	}
}

func TestParseWebhookResponseMethodSkipsForbiddenMethods(t *testing.T) {
	for _, method := range []string{"getMe", "setWebhook", "deleteWebhook", "close", "logout"} {
		if got := parseWebhookResponseMethod([]byte(`{"method":"` + method + `"}`)); got != nil {
			t.Fatalf("parseWebhookResponseMethod(%s) = %+v, want nil", method, got)
		}
	}
}

func TestRunPreservesQueueOrderingAndUsesParallelQueues(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32
	var mu sync.Mutex
	delivered := make([]int, 0, 3)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := active.Add(1)
		for {
			old := maxActive.Load()
			if cur <= old || maxActive.CompareAndSwap(old, cur) {
				break
			}
		}
		body, _ := io.ReadAll(r.Body)
		var update struct {
			Message struct {
				MessageID int `json:"message_id"`
			} `json:"message"`
		}
		_ = json.Unmarshal(body, &update)
		time.Sleep(150 * time.Millisecond)
		mu.Lock()
		delivered = append(delivered, update.Message.MessageID)
		mu.Unlock()
		active.Add(-1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tq := tqueue.New()
	qid := tqueue.QueueID(1)
	exp := int32(time.Now().Add(time.Hour).Unix())
	d := NewDeliverer("test", qid, tq, nil)
	d.bypassSSRF = true // tests use httptest (loopback)
	d.Start(storage.WebhookConfig{URL: srv.URL, MaxConnections: 2}, false)
	d.mu.Lock()
	d.lastSuccessTime = time.Now()
	d.mu.Unlock()
	defer d.Stop()
	_, _ = tq.Push(context.Background(), qid, []byte(`{"update_id":1,"message":{"message_id":1}}`), exp, 10, 0)
	_, _ = tq.Push(context.Background(), qid, []byte(`{"update_id":2,"message":{"message_id":2}}`), exp, 10, 0)
	_, _ = tq.Push(context.Background(), qid, []byte(`{"update_id":3,"message":{"message_id":3}}`), exp, 20, 0)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tq.Size(qid) == 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if tq.Size(qid) != 0 {
		t.Fatalf("queue size = %d, want drained", tq.Size(qid))
	}
	if maxActive.Load() < 2 {
		t.Fatalf("max active deliveries = %d, want parallel delivery", maxActive.Load())
	}
	pos := map[int]int{}
	for i, id := range delivered {
		pos[id] = i
	}
	if pos[1] > pos[2] {
		t.Fatalf("same queue order violated: delivered %v", delivered)
	}
}

func TestDeliver_BodyIsJSON(t *testing.T) {
	var ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	d := newTestDeliverer(t, srv.URL)
	ev := tqueue.Event{ID: tqueue.EventID(1), Data: []byte(`{}`)}
	if forget, _, _ := d.deliver(context.Background(), ev); !forget {
		t.Fatal("expected forget=true for 200")
	}
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestDeliverSendsStoredPayload(t *testing.T) {
	want := []byte(`{"update_id":42,"message":{"message_id":1}}`)
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDeliverer(t, srv.URL)
	if forget, _, _ := d.deliver(context.Background(), tqueue.Event{ID: 99, Data: want}); !forget {
		t.Fatal("expected forget=true for 200")
	}
	select {
	case body := <-got:
		if !bytes.Equal(body, want) {
			t.Fatalf("body = %s, want %s", body, want)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"5", 5},
		{"0", 0},
		{"-1", 0},
		{"abc", 0},
		{"99999", 3600},
	}
	for _, tc := range tests {
		got := parseRetryAfter(tc.in)
		if got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// generateSelfSignedCertPEM creates a self-signed certificate and returns its
// PEM encoding for testing buildCertPool.
func generateSelfSignedCertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "webhook.test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestBuildCertPool(t *testing.T) {
	t.Run("empty cert returns nil", func(t *testing.T) {
		if pool := buildCertPool(nil); pool != nil {
			t.Error("expected nil pool for empty cert")
		}
	})
	t.Run("garbage returns nil", func(t *testing.T) {
		if pool := buildCertPool([]byte("not a certificate")); pool != nil {
			t.Error("expected nil pool for garbage input")
		}
	})
	t.Run("valid self-signed cert returns pool", func(t *testing.T) {
		certPEM := generateSelfSignedCertPEM(t)
		pool := buildCertPool(certPEM)
		if pool == nil {
			t.Fatal("expected non-nil pool for valid cert")
		}
		// Verify the cert chains to the pool (proves it was parsed correctly).
		var block *pem.Block
		block, _ = pem.Decode(certPEM)
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse cert: %v", err)
		}
		if _, err := parsed.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
			t.Errorf("cert does not chain to built pool: %v", err)
		}
	})
	t.Run("non-certificate PEM block ignored", func(t *testing.T) {
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("dummy")})
		if pool := buildCertPool(keyPEM); pool != nil {
			t.Error("expected nil pool when PEM contains no CERTIFICATE blocks")
		}
	})
}
