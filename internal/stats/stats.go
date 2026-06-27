package stats

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Stats holds global and per-bot request/update statistics.
// Thread-safe via atomic counters and RWMutex for the per-bot map.
type Stats struct {
	startTime time.Time

	// Global counters (atomic for lock-free hot path).
	totalRequests   atomic.Int64
	totalOK         atomic.Int64
	totalErrors     atomic.Int64
	totalUpdates    atomic.Int64
	activeBots      atomic.Int64
	totalBots       atomic.Int64
	connectedBots   atomic.Int64

	// Per-bot stats (guarded by mu).
	mu   sync.RWMutex
	bots map[string]*BotStats
}

// BotStats holds per-bot counters.
type BotStats struct {
	Token     string `json:"-"`
	Requests  int64  `json:"requests"`
	OK        int64  `json:"ok"`
	Errors    int64  `json:"errors"`
	Updates   int64  `json:"updates"`
	Connected bool   `json:"connected"`
	LastSeen  int64  `json:"last_seen"` // unix timestamp
}

// New creates a new Stats instance.
func New() *Stats {
	return &Stats{
		startTime: time.Now(),
		bots:      make(map[string]*BotStats),
	}
}

// RecordRequest increments the global and per-bot request counters.
func (s *Stats) RecordRequest(token string, ok bool) {
	s.totalRequests.Add(1)
	if ok {
		s.totalOK.Add(1)
	} else {
		s.totalErrors.Add(1)
	}
	s.mu.Lock()
	bs, exists := s.bots[token]
	if !exists {
		bs = &BotStats{Token: token}
		s.bots[token] = bs
	}
	bs.Requests++
	if ok {
		bs.OK++
	} else {
		bs.Errors++
	}
	bs.LastSeen = time.Now().Unix()
	s.mu.Unlock()
}

// RecordUpdate increments the global and per-bot update counters.
func (s *Stats) RecordUpdate(token string) {
	s.totalUpdates.Add(1)
	s.mu.Lock()
	bs, exists := s.bots[token]
	if !exists {
		bs = &BotStats{Token: token}
		s.bots[token] = bs
	}
	bs.Updates++
	bs.LastSeen = time.Now().Unix()
	s.mu.Unlock()
}

// SetBotConnected tracks bot connection state.
func (s *Stats) SetBotConnected(token string, connected bool) {
	s.mu.Lock()
	bs, exists := s.bots[token]
	if !exists {
		bs = &BotStats{Token: token}
		s.bots[token] = bs
	}
	bs.Connected = connected
	s.mu.Unlock()
	if connected {
		s.connectedBots.Add(1)
	} else {
		s.connectedBots.Add(-1)
	}
}

// RegisterBot increments the total bot count.
func (s *Stats) RegisterBot() {
	s.totalBots.Add(1)
	s.activeBots.Add(1)
}

// UnregisterBot decrements the active bot count.
func (s *Stats) UnregisterBot() {
	s.activeBots.Add(-1)
}

// GlobalStats returns the global statistics snapshot.
type GlobalStats struct {
	Uptime         int64  `json:"uptime_seconds"`
	TotalRequests  int64  `json:"total_requests"`
	TotalOK        int64  `json:"total_ok"`
	TotalErrors    int64  `json:"total_errors"`
	TotalUpdates   int64  `json:"total_updates"`
	ActiveBots     int64  `json:"active_bots"`
	TotalBots      int64  `json:"total_bots"`
	ConnectedBots  int64  `json:"connected_bots"`
}

// GetGlobal returns a snapshot of global statistics.
func (s *Stats) GetGlobal() GlobalStats {
	return GlobalStats{
		Uptime:        int64(time.Since(s.startTime).Seconds()),
		TotalRequests: s.totalRequests.Load(),
		TotalOK:       s.totalOK.Load(),
		TotalErrors:   s.totalErrors.Load(),
		TotalUpdates:  s.totalUpdates.Load(),
		ActiveBots:    s.activeBots.Load(),
		TotalBots:     s.totalBots.Load(),
		ConnectedBots: s.connectedBots.Load(),
	}
}

// GetBotStats returns per-bot statistics.
func (s *Stats) GetBotStats() map[string]*BotStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*BotStats, len(s.bots))
	for k, v := range s.bots {
		bs := *v
		result[k] = &bs
	}
	return result
}

// Handler returns an http.Handler for the /stats endpoint.
func (s *Stats) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"global": s.GetGlobal(),
			"bots":   s.GetBotStats(),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// totalBotAPIMethods is the number of Bot API methods the official server
// implements. Used to compute coverage in MethodCoverage.
const totalBotAPIMethods = 183

// MethodCoverage returns the count of registered methods vs total Bot API methods.
func MethodCoverage(registered int) map[string]any {
	return map[string]any{
		"registered": registered,
		"total":      totalBotAPIMethods,
		"coverage":   float64(registered) / float64(totalBotAPIMethods) * 100,
	}
}
