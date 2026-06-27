package webhook

import (
	"context"
	"net/netip"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

func TestIsForbiddenIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		// Loopback
		{"loopback v4", "127.0.0.1", true},
		{"loopback v4 range", "127.255.255.255", true},
		{"loopback v6", "::1", true},

		// Link-local (cloud metadata)
		{"aws metadata", "169.254.169.254", true},
		{"link-local range", "169.254.0.1", true},
		{"link-local v6", "fe80::1", true},

		// Private ranges
		{"private 10.x", "10.0.0.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 172.31.x", "172.31.255.255", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"private v6 fc00::/7", "fc00::1", true},
		{"private v6 fd00::/7", "fd00::1", true},

		// Unspecified
		{"unspecified v4", "0.0.0.0", true},
		{"unspecified v6", "::", true},

		// Multicast
		{"multicast v4", "224.0.0.1", true},
		{"multicast v6", "ff00::1", true},

		// IPv4-mapped IPv6 of forbidden ranges
		{"4in6 loopback", "::ffff:127.0.0.1", true},
		{"4in6 metadata", "::ffff:169.254.169.254", true},
		{"4in6 private", "::ffff:10.0.0.1", true},

		// Public addresses — allowed
		{"public v4", "8.8.8.8", false},
		{"public v4 2", "1.1.1.1", false},
		{"public v6", "2606:4700:4700::1111", false},

		// Invalid
		{"invalid", "0.0.0.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := netip.ParseAddr(tt.ip)
			if err != nil {
				if got := isForbiddenIP(ip); got != tt.want {
					t.Errorf("isForbiddenIP(parse fail) = %v, want %v", got, tt.want)
				}
				return
			}
			if got := isForbiddenIP(ip); got != tt.want {
				t.Errorf("isForbiddenIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsForbiddenHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"169.254.169.254", true},   // cloud metadata
		{"127.0.0.1", true},         // loopback
		{"10.0.0.1", true},          // private
		{"::1", true},               // loopback v6
		{"8.8.8.8", false},          // public
		{"example.com", false},      // hostname — not checked here
		{"sub.example.com", false},  // hostname
		{"", false},                 // empty — not an IP
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := IsForbiddenHost(tt.host); got != tt.want {
				t.Errorf("IsForbiddenHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestResolveDialHostForConfigRejectsForbiddenIP(t *testing.T) {
	// Deliberately NOT setting bypassSSRF — this is the production path.
	d := NewDeliverer("test", 1, tqueue.New(), nil)
	d.bypassSSRF = false

	cfg := storage.WebhookConfig{
		FixIP:     true,
		IPAddress: "169.254.169.254",
	}
	_, err := d.resolveDialHostForConfig(context.Background(), cfg, "169.254.169.254", "443")
	if err == nil {
		t.Fatal("expected error for forbidden FixIP, got nil")
	}
}

func TestResolveDialHostForConfigRejectsForbiddenPrivateIP(t *testing.T) {
	d := NewDeliverer("test", 1, tqueue.New(), nil)
	d.bypassSSRF = false

	cfg := storage.WebhookConfig{
		FixIP:     true,
		IPAddress: "10.0.0.1",
	}
	_, err := d.resolveDialHostForConfig(context.Background(), cfg, "10.0.0.1", "443")
	if err == nil {
		t.Fatal("expected error for private FixIP, got nil")
	}
}

func TestResolveDialHostForConfigRejectsForbiddenLoopbackIP(t *testing.T) {
	d := NewDeliverer("test", 1, tqueue.New(), nil)
	d.bypassSSRF = false

	cfg := storage.WebhookConfig{
		FixIP:     true,
		IPAddress: "127.0.0.1",
	}
	_, err := d.resolveDialHostForConfig(context.Background(), cfg, "localhost", "443")
	if err == nil {
		t.Fatal("expected error for loopback FixIP, got nil")
	}
}

func TestResolveDialHostForConfigBypassAllowsForbiddenIP(t *testing.T) {
	d := NewDeliverer("test", 1, tqueue.New(), nil)
	d.bypassSSRF = true

	cfg := storage.WebhookConfig{
		FixIP:     true,
		IPAddress: "127.0.0.1",
	}
	got, err := d.resolveDialHostForConfig(context.Background(), cfg, "localhost", "443")
	if err != nil {
		t.Fatalf("expected success with bypass, got %v", err)
	}
	if got != "127.0.0.1" {
		t.Fatalf("got %q, want 127.0.0.1", got)
	}
}

func TestNewDelivererClientHasSSRFTransport(t *testing.T) {
	d := NewDeliverer("test", 1, tqueue.New(), nil)
	if d.client == nil {
		t.Fatal("client is nil")
	}
	// The transport must be non-nil (not the default transport) so that
	// ssrfDialContext is used even before Start() replaces the client.
	if d.client.Transport == nil {
		t.Fatal("expected non-nil transport for SSRF protection")
	}
}
