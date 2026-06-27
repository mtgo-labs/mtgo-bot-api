package webhook

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain verifies no goroutine leaks after all tests complete. The webhook
// package spawns delivery goroutines (Deliverer.run) and httptest servers;
// goleak catches any that aren't cleaned up.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
