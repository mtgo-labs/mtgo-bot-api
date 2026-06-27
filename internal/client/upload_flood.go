package client

import (
	"context"
	"math"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// File-upload flood limiting. Mirrors Client.cpp:13327-13350 (the pre-dispatch
// gate in on_cmd) plus fail_query_flood_limit_exceeded (Client.cpp:16996-17003).
//
// When a request carries files, the server is NOT in local mode, and the total
// uploaded size exceeds uploadFloodThreshold bytes, a per-total-size token
// bucket throttles uploads:
//
//	min_delay = clamp(total_bytes * 1e-7, 0.2s, 0.9s)
//	if backlog > 5s  → 429 Too Many Requests: retry after 60 (after a 3s delay)
//	else last = max(last + min_delay, now - 1.0); if last > now, sleep the
//	     difference then proceed.
//
// The pure math is split into computeFlood so it can be unit-tested without
// time or uploads.

const (
	uploadFloodThreshold  = 100000          // bytes; Client.cpp:13329
	uploadFloodFactor     = 1e-7            // Client.cpp:13332
	uploadFloodMinDelay   = 0.2             // seconds; Client.cpp:13332 clamp lower
	uploadFloodMaxDelay   = 0.9             // seconds; Client.cpp:13332 clamp upper
	uploadFloodMaxBucket  = 1.0             // seconds; max_bucket_volume, Client.cpp:13333
	uploadFloodBacklog    = 5.0             // seconds; Client.cpp:13334 threshold
	uploadFloodRetryAfter = 60              // Client.cpp:17001 set_retry_after_error(60)
	uploadFloodFailDelay  = 3 * time.Second // Client.cpp:16999 SleepActor 3.0
)

// floodOutcome is the decision returned by computeFlood.
type floodOutcome int

const (
	floodProceed floodOutcome = iota // serve the request now
	floodDelay                       // sleep(delay) then serve
	floodLimited                     // sleep 3s then return 429 retry_after=60
)

// computeFlood is the pure token-bucket decision. total is uploaded bytes;
// last is the current bucket value (seconds); now is the current monotonic time
// (seconds). Returns the outcome, the new bucket value to store, and the delay
// (only meaningful for floodDelay).
//
// Mirrors Client.cpp:13332-13349 exactly.
func computeFlood(total int64, last, now float64) (floodOutcome, float64, float64) {
	minDelay := float64(total) * uploadFloodFactor
	if minDelay < uploadFloodMinDelay {
		minDelay = uploadFloodMinDelay
	} else if minDelay > uploadFloodMaxDelay {
		minDelay = uploadFloodMaxDelay
	}
	if last > now+uploadFloodBacklog {
		return floodLimited, last, 0
	}
	newLast := math.Max(last+minDelay, now-uploadFloodMaxBucket)
	if newLast > now {
		return floodDelay, newLast, newLast - now
	}
	return floodProceed, newLast, 0
}

// procStart is a monotonic anchor for the bucket timeline, avoiding wall-clock
// jumps. Buckets store elapsed-seconds-since-procStart.
var procStart = time.Now()

func nowMonotonic() float64 { return time.Since(procStart).Seconds() }

// applyUploadFloodLimit enforces the upload flood gate for a request. It returns
// proceed=true when the handler should run. When proceed=false, status/body are
// the response to write (currently only the 429 flood case).
//
// Locking note: the bucket is mutated under c.mu (matching the reference's
// per-Client actor serialization). The sleeps happen OUTSIDE the lock so they
// do not block connection setup for other requests.
func (c *Client) applyUploadFloodLimit(ctx context.Context, q *server.Query) (status int, body []byte, proceed bool) {
	if c.params.LocalMode || len(q.Files) == 0 {
		return 0, nil, true
	}
	var total int64
	for _, f := range q.Files {
		total += f.Size
	}
	if total <= uploadFloodThreshold {
		return 0, nil, true
	}

	c.mu.Lock()
	last := c.floodBuckets[total]
	now := nowMonotonic()
	outcome, newLast, delay := computeFlood(total, last, now)
	if outcome != floodLimited {
		c.floodBuckets[total] = newLast
	}
	c.mu.Unlock()

	switch outcome {
	case floodProceed:
		return 0, nil, true
	case floodDelay:
		// Sleep the computed delay then proceed (on_cmd force=true path). If the
		// client disconnects mid-delay, proceed anyway; the handler will observe
		// the cancelled context and fail cleanly.
		select {
		case <-time.After(time.Duration(delay * float64(time.Second))):
		case <-ctx.Done():
		}
		return 0, nil, true
	case floodLimited:
		// fail_query_flood_limit_exceeded: sleep 3s, then 429 retry_after=60.
		select {
		case <-time.After(uploadFloodFailDelay):
		case <-ctx.Done():
		}
		return 429, response.Fail(429,
			"Too Many Requests: retry after "+itoa(uploadFloodRetryAfter),
			&response.Parameters{RetryAfter: uploadFloodRetryAfter}), false
	}
	return 0, nil, true
}

// itoa avoids a strconv import in this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
