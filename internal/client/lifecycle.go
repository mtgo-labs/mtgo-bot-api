package client

import (
	"context"
	"fmt"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("logout", (*Client).logout)
	Register("close", (*Client).closeBot)
}

// closingError mirrors get_closing_error + fail_query_closing
// (Client.cpp:16987-17032). Returns ok=false when the client is live and the
// request should be dispatched normally. When ok=true, code/desc/retryAfter are
// the official closing envelope to emit.
//
// Coverage: closing → 500 "Internal Server Error: restart"; logging out with
// queue cleared → 400 "Logged out"; logging out in flight → 401 "Unauthorized".
// The rarer auth-layer cases (is_api_id_invalid_ → 401 "Unauthorized: invalid
// api-id/api-hash"; auth FLOOD_WAIT → 429 retry_after) are deferred — they need
// hooks into the auth layer that don't yet exist.
func (c *Client) closingError() (code int, desc string, retryAfter int, ok bool) {
	if c.closing.Load() {
		return 500, "Internal Server Error: restart", 0, true
	}
	if c.loggingOut.Load() {
		if c.loggedOut.Load() {
			return 400, "Logged out", 0, true
		}
		return 401, "Unauthorized", 0, true
	}
	return 0, "", 0, false
}

// logout logs the bot out from the cloud and clears its update queue.
// Reference: Client.cpp:13309 — sends td_api::logOut (auth.logOut).
func (c *Client) logout(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	c.loggingOut.Store(true) // logging_out_ = true
	_, err := c.rpc.AuthLogOut(ctx)
	if err != nil {
		// Leave loggingOut set: the reference stays in the logging-out state and
		// subsequent requests get fail_query_closing regardless of RPC outcome.
		c.loggedOut.Store(true)
		return nil, rpcError(err)
	}
	// Clear the update queue (reference sets clear_tqueue_ = true).
	c.params.TQueue.Clear(ctx, c.queueID(), 0)
	c.loggedOut.Store(true) // clear_tqueue_ → get_closing_error returns "Logged out"
	return true, nil
}

// closeWindow is the 10-minute window used by the close anti-abuse gate.
// Reference: Client.cpp:13302 `10 * 60`.
const closeWindow = 10 * time.Minute

// closeGateDecision evaluates the close anti-abuse gate
// (Client.cpp:13302-13305). It returns whether the call must be rejected and,
// if so, the retry_after countdown. Pure for deterministic unit testing.
//
// Gate fires only when the client was created more than closeWindow after the
// server boot AND the client is itself younger than closeWindow. retry_after
// then counts down as the client ages.
func closeGateDecision(startTime, serverStart, now time.Time) (limited bool, retryAfter int) {
	// start_time_ > parameters_->start_time_ + 10*60
	if !startTime.After(serverStart.Add(closeWindow)) {
		return false, 0
	}
	elapsed := now.Sub(startTime)
	if elapsed >= closeWindow {
		return false, 0
	}
	// retry_after = static_cast<int>(10*60 - (now - start_time_))
	return true, int((closeWindow - elapsed).Seconds())
}

// closeBot signals the bot to gracefully disconnect.
// Reference: Client.cpp:13301-13308.
func (c *Client) closeBot(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	if limited, retryAfter := closeGateDecision(c.startTime, c.params.StartTime, time.Now()); limited {
		return nil, &Error{
			Code:        429,
			Description: fmt.Sprintf("Too Many Requests: retry after %d", retryAfter),
			Params:      &response.Parameters{RetryAfter: retryAfter},
		}
	}
	// need_close_ = true; do_send_request(td_api::close). Stopping the
	// connection is the Go equivalent of TDLib's graceful close. closing_ is
	// set so subsequent requests get fail_query_closing (500 "restart").
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		go conn.Stop()
	}
	c.closing.Store(true)
	return true, nil
}
