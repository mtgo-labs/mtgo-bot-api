package stats

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestNew verifies the initial state of a freshly created Stats instance.
func TestNew(t *testing.T) {
	s := New()

	// Global counters should be zero.
	g := s.GetGlobal()
	if g.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", g.TotalRequests)
	}
	if g.TotalOK != 0 {
		t.Errorf("TotalOK = %d, want 0", g.TotalOK)
	}
	if g.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d, want 0", g.TotalErrors)
	}
	if g.TotalUpdates != 0 {
		t.Errorf("TotalUpdates = %d, want 0", g.TotalUpdates)
	}
	if g.ActiveBots != 0 {
		t.Errorf("ActiveBots = %d, want 0", g.ActiveBots)
	}
	if g.TotalBots != 0 {
		t.Errorf("TotalBots = %d, want 0", g.TotalBots)
	}
	if g.ConnectedBots != 0 {
		t.Errorf("ConnectedBots = %d, want 0", g.ConnectedBots)
	}

	// Uptime should be small (just created).
	if g.Uptime < 0 {
		t.Errorf("Uptime = %d, want >= 0", g.Uptime)
	}
	if g.Uptime > 5 {
		t.Errorf("Uptime = %d, want <= 5 for a just-created instance", g.Uptime)
	}

	// startTime should be recent.
	if time.Since(s.startTime) > time.Second {
		t.Errorf("startTime too far in the past: %v", s.startTime)
	}

	// Per-bot map should be empty and non-nil.
	if s.bots == nil {
		t.Fatal("bots map is nil")
	}
	if len(s.bots) != 0 {
		t.Errorf("bots map len = %d, want 0", len(s.bots))
	}
	if len(s.GetBotStats()) != 0 {
		t.Errorf("GetBotStats() len != 0 for new instance")
	}
}

// TestRecordRequest_OK verifies that successful requests increment the right counters.
func TestRecordRequest_OK(t *testing.T) {
	s := New()
	token := "123456:ABC"

	for i := 0; i < 5; i++ {
		s.RecordRequest(token, true)
	}

	g := s.GetGlobal()
	if g.TotalRequests != 5 {
		t.Errorf("TotalRequests = %d, want 5", g.TotalRequests)
	}
	if g.TotalOK != 5 {
		t.Errorf("TotalOK = %d, want 5", g.TotalOK)
	}
	if g.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d, want 0", g.TotalErrors)
	}

	bots := s.GetBotStats()
	bs, ok := bots[token]
	if !ok {
		t.Fatalf("bot %q not in stats map", token)
	}
	if bs.Requests != 5 {
		t.Errorf("Requests = %d, want 5", bs.Requests)
	}
	if bs.OK != 5 {
		t.Errorf("OK = %d, want 5", bs.OK)
	}
	if bs.Errors != 0 {
		t.Errorf("Errors = %d, want 0", bs.Errors)
	}
}

// TestRecordRequest_Errors verifies that failed requests increment the error counters.
func TestRecordRequest_Errors(t *testing.T) {
	s := New()
	token := "bot-error"

	for i := 0; i < 3; i++ {
		s.RecordRequest(token, false)
	}

	g := s.GetGlobal()
	if g.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", g.TotalRequests)
	}
	if g.TotalOK != 0 {
		t.Errorf("TotalOK = %d, want 0", g.TotalOK)
	}
	if g.TotalErrors != 3 {
		t.Errorf("TotalErrors = %d, want 3", g.TotalErrors)
	}

	bots := s.GetBotStats()
	bs, ok := bots[token]
	if !ok {
		t.Fatalf("bot %q not in stats map", token)
	}
	if bs.Requests != 3 {
		t.Errorf("Requests = %d, want 3", bs.Requests)
	}
	if bs.Errors != 3 {
		t.Errorf("Errors = %d, want 3", bs.Errors)
	}
	if bs.OK != 0 {
		t.Errorf("OK = %d, want 0", bs.OK)
	}
}

// TestRecordRequest_MixedOKAndErrors verifies a mix of ok and error requests.
func TestRecordRequest_MixedOKAndErrors(t *testing.T) {
	s := New()
	token := "bot-mixed"

	s.RecordRequest(token, true)
	s.RecordRequest(token, false)
	s.RecordRequest(token, true)
	s.RecordRequest(token, false)

	g := s.GetGlobal()
	if g.TotalRequests != 4 {
		t.Fatalf("TotalRequests = %d, want 4", g.TotalRequests)
	}
	if g.TotalOK != 2 {
		t.Errorf("TotalOK = %d, want 2", g.TotalOK)
	}
	if g.TotalErrors != 2 {
		t.Errorf("TotalErrors = %d, want 2", g.TotalErrors)
	}

	bots := s.GetBotStats()
	bs := bots[token]
	if bs.Requests != 4 {
		t.Errorf("Requests = %d, want 4", bs.Requests)
	}
	if bs.OK != 2 {
		t.Errorf("OK = %d, want 2", bs.OK)
	}
	if bs.Errors != 2 {
		t.Errorf("Errors = %d, want 2", bs.Errors)
	}
}

// TestRecordRequest_MultipleBots verifies per-bot isolation across different tokens.
func TestRecordRequest_MultipleBots(t *testing.T) {
	s := New()

	t1, t2, t3 := "token1", "token2", "token3"
	s.RecordRequest(t1, true)
	s.RecordRequest(t1, true)
	s.RecordRequest(t2, false)
	s.RecordRequest(t3, true)
	s.RecordRequest(t3, true)
	s.RecordRequest(t3, false)

	bots := s.GetBotStats()
	if len(bots) != 3 {
		t.Fatalf("expected 3 bots, got %d", len(bots))
	}

	if bots[t1].Requests != 2 || bots[t1].OK != 2 || bots[t1].Errors != 0 {
		t.Errorf("token1 stats: %+v", bots[t1])
	}
	if bots[t2].Requests != 1 || bots[t2].OK != 0 || bots[t2].Errors != 1 {
		t.Errorf("token2 stats: %+v", bots[t2])
	}
	if bots[t3].Requests != 3 || bots[t3].OK != 2 || bots[t3].Errors != 1 {
		t.Errorf("token3 stats: %+v", bots[t3])
	}

	g := s.GetGlobal()
	if g.TotalRequests != 6 {
		t.Errorf("TotalRequests = %d, want 6", g.TotalRequests)
	}
}

// TestRecordRequest_SetsLastSeen verifies that LastSeen is set on each request.
func TestRecordRequest_SetsLastSeen(t *testing.T) {
	s := New()
	token := "bot-lastseen"
	before := time.Now().Unix() - 1

	s.RecordRequest(token, true)

	bots := s.GetBotStats()
	bs := bots[token]
	if bs.LastSeen < before {
		t.Errorf("LastSeen = %d, want >= %d", bs.LastSeen, before)
	}
	if bs.LastSeen > time.Now().Unix()+1 {
		t.Errorf("LastSeen = %d, in the future", bs.LastSeen)
	}
}

// TestRecordRequest_Table is a table-driven test combining ok/error scenarios.
func TestRecordRequest_Table(t *testing.T) {
	tests := []struct {
		name       string
		okCount    int
		errCount   int
		wantOK     int64
		wantErr    int64
		wantTotal  int64
	}{
		{"all_ok", 5, 0, 5, 0, 5},
		{"all_errors", 0, 3, 0, 3, 3},
		{"mixed", 3, 2, 3, 2, 5},
		{"none", 0, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			token := "bot-" + tt.name
			for i := 0; i < tt.okCount; i++ {
				s.RecordRequest(token, true)
			}
			for i := 0; i < tt.errCount; i++ {
				s.RecordRequest(token, false)
			}

			g := s.GetGlobal()
			if g.TotalRequests != tt.wantTotal {
				t.Errorf("TotalRequests = %d, want %d", g.TotalRequests, tt.wantTotal)
			}
			if g.TotalOK != tt.wantOK {
				t.Errorf("TotalOK = %d, want %d", g.TotalOK, tt.wantOK)
			}
			if g.TotalErrors != tt.wantErr {
				t.Errorf("TotalErrors = %d, want %d", g.TotalErrors, tt.wantErr)
			}

		bs := s.GetBotStats()[token]
		if bs == nil {
			if tt.wantTotal > 0 {
				t.Fatalf("bot missing from stats")
			}
			return
		}
		if bs.Requests != tt.wantTotal {
			t.Errorf("Requests = %d, want %d", bs.Requests, tt.wantTotal)
		}
		})
	}
}

// TestRecordUpdate increments the update counters and per-bot update field.
func TestRecordUpdate(t *testing.T) {
	s := New()
	token := "bot-updates"

	for i := 0; i < 7; i++ {
		s.RecordUpdate(token)
	}

	g := s.GetGlobal()
	if g.TotalUpdates != 7 {
		t.Errorf("TotalUpdates = %d, want 7", g.TotalUpdates)
	}
	// Updates should not affect request counters.
	if g.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", g.TotalUpdates)
	}

	bs := s.GetBotStats()[token]
	if bs.Updates != 7 {
		t.Errorf("Updates = %d, want 7", bs.Updates)
	}
	// Per-bot update should not affect Requests/OK/Errors.
	if bs.Requests != 0 || bs.OK != 0 || bs.Errors != 0 {
		t.Errorf("update incremented request fields: %+v", bs)
	}
}

// TestRecordUpdate_SetsLastSeen verifies LastSeen is set on update.
func TestRecordUpdate_SetsLastSeen(t *testing.T) {
	s := New()
	token := "bot-upd-lastseen"
	before := time.Now().Unix() - 1

	s.RecordUpdate(token)

	bs := s.GetBotStats()[token]
	if bs.LastSeen < before {
		t.Errorf("LastSeen = %d, want >= %d", bs.LastSeen, before)
	}
}

// TestRecordUpdate_MultipleBots verifies per-bot isolation for updates.
func TestRecordUpdate_MultipleBots(t *testing.T) {
	s := New()
	t1, t2 := "t1", "t2"

	s.RecordUpdate(t1)
	s.RecordUpdate(t1)
	s.RecordUpdate(t2)

	bots := s.GetBotStats()
	if bots[t1].Updates != 2 {
		t.Errorf("t1 Updates = %d, want 2", bots[t1].Updates)
	}
	if bots[t2].Updates != 1 {
		t.Errorf("t2 Updates = %d, want 1", bots[t2].Updates)
	}

	g := s.GetGlobal()
	if g.TotalUpdates != 3 {
		t.Errorf("TotalUpdates = %d, want 3", g.TotalUpdates)
	}
}

// TestSetBotConnected_Connect verifies connecting a bot sets Connected and increments connectedBots.
func TestSetBotConnected_Connect(t *testing.T) {
	s := New()
	token := "bot-conn"

	s.SetBotConnected(token, true)

	bs := s.GetBotStats()[token]
	if !bs.Connected {
		t.Errorf("Connected = false, want true")
	}

	g := s.GetGlobal()
	if g.ConnectedBots != 1 {
		t.Errorf("ConnectedBots = %d, want 1", g.ConnectedBots)
	}
}

// TestSetBotConnected_Disconnect verifies disconnecting decrements connectedBots.
func TestSetBotConnected_Disconnect(t *testing.T) {
	s := New()
	token := "bot-conn-disc"

	s.SetBotConnected(token, true)
	s.SetBotConnected(token, false)

	bs := s.GetBotStats()[token]
	if bs.Connected {
		t.Errorf("Connected = true, want false")
	}

	g := s.GetGlobal()
	if g.ConnectedBots != 0 {
		t.Errorf("ConnectedBots = %d, want 0", g.ConnectedBots)
	}
}

// TestSetBotConnected_MultipleBots verifies connectedBots tracks multiple bots.
func TestSetBotConnected_MultipleBots(t *testing.T) {
	s := New()

	s.SetBotConnected("a", true)
	s.SetBotConnected("b", true)
	s.SetBotConnected("c", true)

	g := s.GetGlobal()
	if g.ConnectedBots != 3 {
		t.Errorf("ConnectedBots = %d, want 3", g.ConnectedBots)
	}

	s.SetBotConnected("b", false)
	g = s.GetGlobal()
	if g.ConnectedBots != 2 {
		t.Errorf("ConnectedBots = %d, want 2", g.ConnectedBots)
	}

	s.SetBotConnected("a", false)
	s.SetBotConnected("c", false)
	g = s.GetGlobal()
	if g.ConnectedBots != 0 {
		t.Errorf("ConnectedBots = %d, want 0", g.ConnectedBots)
	}
}

// TestSetBotConnected_CreatesBotIfMissing verifies SetBotConnected creates the bot entry if it doesn't exist.
func TestSetBotConnected_CreatesBotIfMissing(t *testing.T) {
	s := New()
	token := "new-bot"

	s.SetBotConnected(token, true)

	bots := s.GetBotStats()
	bs, ok := bots[token]
	if !ok {
		t.Fatalf("bot %q not created by SetBotConnected", token)
	}
	if !bs.Connected {
		t.Errorf("Connected = false, want true")
	}
	if bs.Requests != 0 || bs.Updates != 0 || bs.OK != 0 || bs.Errors != 0 {
		t.Errorf("unexpected counters for new bot: %+v", bs)
	}
}

// TestSetBotConnected_Toggle verifies repeated connect/disconnect cycles.
func TestSetBotConnected_Toggle(t *testing.T) {
	s := New()
	token := "bot-toggle"

	for i := 0; i < 5; i++ {
		s.SetBotConnected(token, true)
		s.SetBotConnected(token, false)
	}

	g := s.GetGlobal()
	// Each pair should net to zero.
	if g.ConnectedBots != 0 {
		t.Errorf("ConnectedBots = %d, want 0 after balanced toggles", g.ConnectedBots)
	}
}

// TestRegisterBot verifies RegisterBot increments both activeBots and totalBots.
func TestRegisterBot(t *testing.T) {
	s := New()

	s.RegisterBot()
	g := s.GetGlobal()
	if g.ActiveBots != 1 {
		t.Errorf("ActiveBots = %d, want 1", g.ActiveBots)
	}
	if g.TotalBots != 1 {
		t.Errorf("TotalBots = %d, want 1", g.TotalBots)
	}

	s.RegisterBot()
	s.RegisterBot()
	g = s.GetGlobal()
	if g.ActiveBots != 3 {
		t.Errorf("ActiveBots = %d, want 3", g.ActiveBots)
	}
	if g.TotalBots != 3 {
		t.Errorf("TotalBots = %d, want 3", g.TotalBots)
	}
}

// TestUnregisterBot verifies UnregisterBot decrements activeBots but not totalBots.
func TestUnregisterBot(t *testing.T) {
	s := New()

	s.RegisterBot()
	s.RegisterBot()
	s.UnregisterBot()

	g := s.GetGlobal()
	if g.ActiveBots != 1 {
		t.Errorf("ActiveBots = %d, want 1", g.ActiveBots)
	}
	// totalBots should not decrease on unregister.
	if g.TotalBots != 2 {
		t.Errorf("TotalBots = %d, want 2", g.TotalBots)
	}
}

// TestRegisterUnregisterLifecycle verifies a full register/unregister lifecycle.
func TestRegisterUnregisterLifecycle(t *testing.T) {
	s := New()

	// Register 5 bots.
	for i := 0; i < 5; i++ {
		s.RegisterBot()
	}
	// Unregister 2.
	s.UnregisterBot()
	s.UnregisterBot()

	g := s.GetGlobal()
	if g.ActiveBots != 3 {
		t.Errorf("ActiveBots = %d, want 3", g.ActiveBots)
	}
	if g.TotalBots != 5 {
		t.Errorf("TotalBots = %d, want 5", g.TotalBots)
	}
}

// TestGetGlobal verifies the global snapshot reflects accumulated state.
func TestGetGlobal(t *testing.T) {
	s := New()

	// Seed some activity.
	s.RegisterBot()
	s.RegisterBot()
	s.SetBotConnected("bot1", true)
	s.SetBotConnected("bot2", true)
	s.RecordRequest("bot1", true)
	s.RecordRequest("bot1", false)
	s.RecordUpdate("bot2")

	// Sleep briefly so uptime is at least 1 second.
	time.Sleep(1100 * time.Millisecond)

	g := s.GetGlobal()

	if g.TotalRequests != 2 {
		t.Errorf("TotalRequests = %d, want 2", g.TotalRequests)
	}
	if g.TotalOK != 1 {
		t.Errorf("TotalOK = %d, want 1", g.TotalOK)
	}
	if g.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", g.TotalErrors)
	}
	if g.TotalUpdates != 1 {
		t.Errorf("TotalUpdates = %d, want 1", g.TotalUpdates)
	}
	if g.ActiveBots != 2 {
		t.Errorf("ActiveBots = %d, want 2", g.ActiveBots)
	}
	if g.TotalBots != 2 {
		t.Errorf("TotalBots = %d, want 2", g.TotalBots)
	}
	if g.ConnectedBots != 2 {
		t.Errorf("ConnectedBots = %d, want 2", g.ConnectedBots)
	}
	if g.Uptime < 1 {
		t.Errorf("Uptime = %d, want >= 1 after sleep", g.Uptime)
	}
}

// TestGetGlobal_JSONTags verifies the JSON field names on GlobalStats.
func TestGetGlobal_JSONTags(t *testing.T) {
	s := New()
	s.RecordRequest("b", true)

	g := s.GetGlobal()
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wantKeys := []string{
		"uptime_seconds",
		"total_requests",
		"total_ok",
		"total_errors",
		"total_updates",
		"active_bots",
		"total_bots",
		"connected_bots",
	}
	for _, k := range wantKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("missing JSON key %q in global stats", k)
		}
	}
}

// TestGetBotStats_ReturnsCopy verifies GetBotStats returns a copy, not the internal map.
func TestGetBotStats_ReturnsCopy(t *testing.T) {
	s := New()
	s.RecordRequest("bot1", true)

	bots := s.GetBotStats()

	// Mutate the returned map — should not affect internal state.
	bots["injected"] = &BotStats{Token: "injected", Requests: 999}
	delete(bots, "bot1")

	internal := s.GetBotStats()
	if _, ok := internal["injected"]; ok {
		t.Error("mutating returned map leaked injected bot into internal state")
	}
	if _, ok := internal["bot1"]; !ok {
		t.Error("deleting from returned map removed bot1 from internal state")
	}
}

// TestGetBotStats_PointersAreCopies verifies that the BotStats pointers point to copies,
// so mutating them does not affect internal state.
func TestGetBotStats_PointersAreCopies(t *testing.T) {
	s := New()
	s.RecordRequest("bot1", true)

	bots := s.GetBotStats()
	bots["bot1"].Requests = 999
	bots["bot1"].OK = 999
	bots["bot1"].Connected = true

	// Re-fetch — internal state should be unchanged.
	bots2 := s.GetBotStats()
	bs := bots2["bot1"]
	if bs.Requests != 1 {
		t.Errorf("Requests = %d, want 1 (copy was mutated)", bs.Requests)
	}
	if bs.OK != 1 {
		t.Errorf("OK = %d, want 1 (copy was mutated)", bs.OK)
	}
	if bs.Connected {
		t.Errorf("Connected = true, want false (copy was mutated)")
	}
}

// TestGetBotStats_Empty verifies an empty stats instance returns an empty (non-nil) map.
func TestGetBotStats_Empty(t *testing.T) {
	s := New()
	bots := s.GetBotStats()
	if bots == nil {
		t.Fatal("GetBotStats returned nil for new instance")
	}
	if len(bots) != 0 {
		t.Errorf("len = %d, want 0", len(bots))
	}
}

// TestGetBotStats_TokenFieldIsJSONOmitted verifies the Token field has json:"-" tag.
func TestGetBotStats_TokenFieldIsJSONOmitted(t *testing.T) {
	s := New()
	s.RecordRequest("secret-token", true)

	bots := s.GetBotStats()
	data, err := json.Marshal(bots)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range raw {
		if _, ok := m["Token"]; ok {
			t.Errorf("Token field should be omitted by json:\"-\" tag")
		}
		if _, ok := m["token"]; ok {
			t.Errorf("token field should be omitted by json:\"-\" tag")
		}
	}
}

// TestHandler_ServesJSON verifies Handler returns valid JSON at /stats with correct structure.
func TestHandler_ServesJSON(t *testing.T) {
	s := New()
	s.RecordRequest("bot1", true)
	s.RecordUpdate("bot1")

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\nbody: %s", err, w.Body.String())
	}

	global, ok := resp["global"]
	if !ok {
		t.Fatal("response missing 'global' key")
	}
	globalMap, ok := global.(map[string]any)
	if !ok {
		t.Fatalf("global is not a map: %T", global)
	}
	if globalMap["total_requests"].(float64) != 1 {
		t.Errorf("global.total_requests = %v, want 1", globalMap["total_requests"])
	}

	bots, ok := resp["bots"]
	if !ok {
		t.Fatal("response missing 'bots' key")
	}
	botsMap, ok := bots.(map[string]any)
	if !ok {
		t.Fatalf("bots is not a map: %T", bots)
	}
	if len(botsMap) != 1 {
		t.Errorf("bots map len = %d, want 1", len(botsMap))
	}
}

// TestHandler_EmptyStats verifies the handler works with no activity.
func TestHandler_EmptyStats(t *testing.T) {
	s := New()

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	global := resp["global"].(map[string]any)
	if global["total_requests"].(float64) != 0 {
		t.Errorf("total_requests = %v, want 0", global["total_requests"])
	}
}

// TestHandler_PostMethod verifies the handler does not restrict to GET (it's a generic handler).
func TestHandler_PostMethod(t *testing.T) {
	s := New()

	req := httptest.NewRequest(http.MethodPost, "/stats", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	// The handler does not filter by method, so it should still return 200.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (handler should not filter by method)", w.Code, http.StatusOK)
	}
}

// TestHandler_ReturnsHandler verifies Handler returns a non-nil http.Handler.
func TestHandler_ReturnsHandler(t *testing.T) {
	s := New()
	h := s.Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}

	// Verify it's usable as http.HandlerFunc.
	if _, ok := h.(http.HandlerFunc); !ok {
		// It doesn't have to be HandlerFunc specifically, just usable.
		// Verify it can serve a request.
		req := httptest.NewRequest(http.MethodGet, "/stats", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	}
}

// TestMethodCoverage verifies the method coverage calculation.
func TestMethodCoverage(t *testing.T) {
	tests := []struct {
		name          string
		registered    int
		wantRegistered int
		wantTotal     int
		wantCoverage  float64
	}{
		{"zero", 0, 0, 183, 0.0},
		{"half", 91, 91, 183, 91.0 / 183.0 * 100},
		{"full", 183, 183, 183, 100.0},
		{"over", 200, 200, 183, 200.0 / 183.0 * 100},
		{"partial", 50, 50, 183, 50.0 / 183.0 * 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MethodCoverage(tt.registered)

			if m["registered"].(int) != tt.wantRegistered {
				t.Errorf("registered = %v, want %d", m["registered"], tt.wantRegistered)
			}
			if m["total"].(int) != tt.wantTotal {
				t.Errorf("total = %v, want %d", m["total"], tt.wantTotal)
			}
			coverage, ok := m["coverage"].(float64)
			if !ok {
				t.Fatalf("coverage is not float64: %T", m["coverage"])
			}
			// Use a tolerance for float comparison.
			if abs := coverage - tt.wantCoverage; abs > 0.001 || abs < -0.001 {
				t.Errorf("coverage = %f, want %f", coverage, tt.wantCoverage)
			}
		})
	}
}

// TestMethodCoverage_ReturnsAllKeys verifies all expected keys are present.
func TestMethodCoverage_ReturnsAllKeys(t *testing.T) {
	m := MethodCoverage(100)

	wantKeys := []string{"registered", "total", "coverage"}
	for _, k := range wantKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if len(m) != len(wantKeys) {
		t.Errorf("map has %d keys, want %d", len(m), len(wantKeys))
	}
}

// TestConcurrent_RecordRequest verifies atomic correctness under concurrent access.
func TestConcurrent_RecordRequest(t *testing.T) {
	s := New()

	const goroutines = 100
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			token := "shared-bot"
			if id%2 == 0 {
				token = "even-bot"
			}
			for j := 0; j < perGoroutine; j++ {
				s.RecordRequest(token, id%3 != 0)
			}
		}(i)
	}
	wg.Wait()

	totalExpected := int64(goroutines * perGoroutine)
	g := s.GetGlobal()
	if g.TotalRequests != totalExpected {
		t.Errorf("TotalRequests = %d, want %d", g.TotalRequests, totalExpected)
	}
	if g.TotalOK+g.TotalErrors != totalExpected {
		t.Errorf("TotalOK(%d) + TotalErrors(%d) = %d, want %d",
			g.TotalOK, g.TotalErrors, g.TotalOK+g.TotalErrors, totalExpected)
	}
}

// TestConcurrent_RecordUpdate verifies update counters under concurrent access.
func TestConcurrent_RecordUpdate(t *testing.T) {
	s := New()

	const goroutines = 100
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			token := "bot-1"
			if id%5 == 0 {
				token = "bot-2"
			}
			for j := 0; j < perGoroutine; j++ {
				s.RecordUpdate(token)
			}
		}(i)
	}
	wg.Wait()

	totalExpected := int64(goroutines * perGoroutine)
	g := s.GetGlobal()
	if g.TotalUpdates != totalExpected {
		t.Errorf("TotalUpdates = %d, want %d", g.TotalUpdates, totalExpected)
	}
}

// TestConcurrent_MixedOperations verifies all operations together under concurrency.
func TestConcurrent_MixedOperations(t *testing.T) {
	s := New()

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	// Half do requests, half do updates, some toggle connection, some register/unregister.
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.RecordRequest("bot-a", j%2 == 0)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.RecordUpdate("bot-b")
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.SetBotConnected("bot-c", j%2 == 0)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.RegisterBot()
				s.UnregisterBot()
			}
		}()
	}
	wg.Wait()

	g := s.GetGlobal()

	wantRequests := int64(goroutines * iterations)
	if g.TotalRequests != wantRequests {
		t.Errorf("TotalRequests = %d, want %d", g.TotalRequests, wantRequests)
	}

	wantUpdates := int64(goroutines * iterations)
	if g.TotalUpdates != wantUpdates {
		t.Errorf("TotalUpdates = %d, want %d", g.TotalUpdates, wantUpdates)
	}

	// Each register+unregister pair nets zero for activeBots.
	if g.ActiveBots != 0 {
		t.Errorf("ActiveBots = %d, want 0", g.ActiveBots)
	}
	// totalBots should equal goroutines * iterations.
	wantTotalBots := int64(goroutines * iterations)
	if g.TotalBots != wantTotalBots {
		t.Errorf("TotalBots = %d, want %d", g.TotalBots, wantTotalBots)
	}

	// Verify per-bot map consistency — GetBotStats should not panic and return 3 bots.
	bots := s.GetBotStats()
	if len(bots) != 3 {
		t.Errorf("bots map len = %d, want 3", len(bots))
	}
}

// TestConcurrent_GetGlobalAndBotStats verifies reads are safe under concurrent writes.
func TestConcurrent_GetGlobalAndBotStats(t *testing.T) {
	s := New()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Writer goroutines.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					s.RecordRequest("bot", true)
					s.RecordUpdate("bot")
				}
			}
		}(i)
	}

	// Reader goroutines — just verify no panics.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				s.GetGlobal()
				s.GetBotStats()
			}
		}()
	}

	// Let readers finish (they have bounded loops), then stop writers.
	// Actually, readers and writers run concurrently; we stop after a short duration.
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Just verify we got here without panicking.
	g := s.GetGlobal()
	if g.TotalRequests == 0 {
		t.Error("no requests recorded during concurrent test")
	}
}

// TestConcurrent_GetBotStatsConsistency verifies that concurrent reads produce consistent snapshots.
func TestConcurrent_GetBotStatsConsistency(t *testing.T) {
	s := New()
	s.RecordRequest("bot1", true)
	s.RecordRequest("bot2", true)

	var wg sync.WaitGroup
	const readers = 20

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				bots := s.GetBotStats()
				// Each snapshot should have at least 2 bots.
				if len(bots) < 2 {
					t.Errorf("snapshot missing bots: len = %d", len(bots))
				}
			}
		}()
	}
	wg.Wait()
}

// TestIntegration_FullWorkflow exercises all methods together in a realistic scenario.
func TestIntegration_FullWorkflow(t *testing.T) {
	s := New()

	// Register two bots.
	s.RegisterBot()
	s.RegisterBot()

	// Connect both.
	s.SetBotConnected("token-a", true)
	s.SetBotConnected("token-b", true)

	// Bot A makes 10 requests (8 ok, 2 err) and receives 5 updates.
	for i := 0; i < 8; i++ {
		s.RecordRequest("token-a", true)
	}
	for i := 0; i < 2; i++ {
		s.RecordRequest("token-a", false)
	}
	for i := 0; i < 5; i++ {
		s.RecordUpdate("token-a")
	}

	// Bot B makes 3 requests (all ok) and receives 2 updates.
	for i := 0; i < 3; i++ {
		s.RecordRequest("token-b", true)
	}
	for i := 0; i < 2; i++ {
		s.RecordUpdate("token-b")
	}

	// Disconnect bot B.
	s.SetBotConnected("token-b", false)

	// Unregister bot B.
	s.UnregisterBot()

	// Verify global snapshot.
	g := s.GetGlobal()
	if g.TotalRequests != 13 {
		t.Errorf("TotalRequests = %d, want 13", g.TotalRequests)
	}
	if g.TotalOK != 11 {
		t.Errorf("TotalOK = %d, want 11", g.TotalOK)
	}
	if g.TotalErrors != 2 {
		t.Errorf("TotalErrors = %d, want 2", g.TotalErrors)
	}
	if g.TotalUpdates != 7 {
		t.Errorf("TotalUpdates = %d, want 7", g.TotalUpdates)
	}
	if g.TotalBots != 2 {
		t.Errorf("TotalBots = %d, want 2", g.TotalBots)
	}
	if g.ActiveBots != 1 {
		t.Errorf("ActiveBots = %d, want 1", g.ActiveBots)
	}
	if g.ConnectedBots != 1 {
		t.Errorf("ConnectedBots = %d, want 1", g.ConnectedBots)
	}

	// Verify per-bot stats.
	bots := s.GetBotStats()
	bsA := bots["token-a"]
	if bsA.Requests != 10 {
		t.Errorf("bot-a Requests = %d, want 10", bsA.Requests)
	}
	if bsA.OK != 8 {
		t.Errorf("bot-a OK = %d, want 8", bsA.OK)
	}
	if bsA.Errors != 2 {
		t.Errorf("bot-a Errors = %d, want 2", bsA.Errors)
	}
	if bsA.Updates != 5 {
		t.Errorf("bot-a Updates = %d, want 5", bsA.Updates)
	}
	if !bsA.Connected {
		t.Errorf("bot-a Connected = false, want true")
	}

	bsB := bots["token-b"]
	if bsB.Requests != 3 {
		t.Errorf("bot-b Requests = %d, want 3", bsB.Requests)
	}
	if bsB.Updates != 2 {
		t.Errorf("bot-b Updates = %d, want 2", bsB.Updates)
	}
	if bsB.Connected {
		t.Errorf("bot-b Connected = true, want false")
	}

	// Verify the HTTP handler works with this state.
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("handler status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	respGlobal := resp["global"].(map[string]any)
	if respGlobal["total_requests"].(float64) != 13 {
		t.Errorf("handler global.total_requests = %v, want 13", respGlobal["total_requests"])
	}
	respBots := resp["bots"].(map[string]any)
	if len(respBots) != 2 {
		t.Errorf("handler bots len = %d, want 2", len(respBots))
	}
}
