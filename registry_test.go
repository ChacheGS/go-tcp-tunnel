package tunnel

import (
	"testing"

	"github.com/ChacheGS/go-tcp-tunnel/id"
)

func newTestID(data string) id.ID {
	return id.New([]byte(data))
}

func TestRegistry_SubscribeAndIsSubscribed(t *testing.T) {
	t.Parallel()

	r := newRegistry(nil)
	identifier := newTestID("client1")

	if r.IsSubscribed(identifier) {
		t.Fatal("should not be subscribed initially")
	}

	r.Subscribe(identifier)
	if !r.IsSubscribed(identifier) {
		t.Fatal("should be subscribed after Subscribe")
	}

	// Idempotent re-subscribe
	r.Subscribe(identifier)
	if !r.IsSubscribed(identifier) {
		t.Fatal("should still be subscribed after re-subscribe")
	}
}

func TestRegistry_Unsubscribe(t *testing.T) {
	t.Parallel()

	r := newRegistry(nil)
	identifier := newTestID("client1")

	// Unsubscribe unknown returns nil
	if item := r.Unsubscribe(identifier); item != nil {
		t.Fatal("expected nil for unknown client")
	}

	r.Subscribe(identifier)
	item := r.Unsubscribe(identifier)
	if item == nil {
		t.Fatal("expected non-nil item")
	}

	if r.IsSubscribed(identifier) {
		t.Fatal("should not be subscribed after Unsubscribe")
	}
}

func TestRegistry_Subscriber(t *testing.T) {
	t.Parallel()

	r := newRegistry(nil)
	identifier := newTestID("client1")

	r.Subscribe(identifier)

	hosts := []string{"example.com:80"}
	item := &RegistryItem{Hosts: hosts}
	if err := r.set(item, identifier); err != nil {
		t.Fatal(err)
	}

	got, ok := r.Subscriber("example.com:80")
	if !ok {
		t.Fatal("expected to find subscriber")
	}
	if got != identifier {
		t.Fatalf("expected identifier %v, got %v", identifier, got)
	}

	// Unknown host
	_, ok = r.Subscriber("unknown.com:80")
	if ok {
		t.Fatal("should not find subscriber for unknown host")
	}
}

func TestRegistry_Set(t *testing.T) {
	t.Parallel()

	r := newRegistry(nil)
	identifier := newTestID("client1")
	identifier2 := newTestID("client2")

	// Error: not subscribed
	item := &RegistryItem{Hosts: []string{"example.com:80"}}
	if err := r.set(item, identifier); err != errClientNotSubscribed {
		t.Fatalf("expected errClientNotSubscribed, got %v", err)
	}

	r.Subscribe(identifier)

	// Success
	if err := r.set(item, identifier); err != nil {
		t.Fatal(err)
	}

	// Error: overwrite
	item2 := &RegistryItem{Hosts: []string{"other.com:80"}}
	if err := r.set(item2, identifier); err == nil {
		t.Fatal("expected overwrite error")
	}

	// Error: host occupied
	r.Subscribe(identifier2)
	item3 := &RegistryItem{Hosts: []string{"example.com:80"}}
	if err := r.set(item3, identifier2); err == nil {
		t.Fatal("expected host occupied error")
	}
}

func TestRegistry_Clear(t *testing.T) {
	t.Parallel()

	r := newRegistry(nil)
	identifier := newTestID("client1")

	r.Subscribe(identifier)

	item := &RegistryItem{Hosts: []string{"example.com:80"}}
	if err := r.set(item, identifier); err != nil {
		t.Fatal(err)
	}

	cleared := r.clear(identifier)
	if cleared == nil {
		t.Fatal("expected non-nil cleared item")
	}

	// Host should be removed
	_, ok := r.Subscriber("example.com:80")
	if ok {
		t.Fatal("host should be cleared")
	}

	// Second clear returns nil (already void)
	if r.clear(identifier) != nil {
		t.Fatal("second clear should return nil")
	}
}

func TestRegistry_UnsubscribeClearsHosts(t *testing.T) {
	t.Parallel()

	r := newRegistry(nil)
	identifier := newTestID("client1")

	r.Subscribe(identifier)

	item := &RegistryItem{Hosts: []string{"example.com:80", "other.com:443"}}
	if err := r.set(item, identifier); err != nil {
		t.Fatal(err)
	}

	// Verify hosts are set
	if _, ok := r.Subscriber("example.com:80"); !ok {
		t.Fatal("host should be set")
	}

	r.Unsubscribe(identifier)

	// Client should be removed
	if r.IsSubscribed(identifier) {
		t.Fatal("should not be subscribed after Unsubscribe")
	}

	// Hosts should be cleaned up
	if _, ok := r.Subscriber("example.com:80"); ok {
		t.Fatal("host should be removed after unsubscribe")
	}
	if _, ok := r.Subscriber("other.com:443"); ok {
		t.Fatal("host should be removed after unsubscribe")
	}
}

func TestTrimPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"example.com:80", "example.com"},
		{"example.com:443", "example.com"},
		{"127.0.0.1:8080", "127.0.0.1"},
		{"example.com", "example.com"}, // no port
		{"[::1]:80", "::1"},
	}

	for _, tt := range tests {
		result := trimPort(tt.input)
		if result != tt.expected {
			t.Errorf("trimPort(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
