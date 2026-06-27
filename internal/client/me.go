package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("getme", (*Client).getMe)
}

// getMe implements the Bot API getMe method via the raw tg layer: it calls
// UsersGetFullUser with InputUserSelf (the same raw call telegram.Client.GetMe
// uses internally) and decodes the *tg.UsersUserFull result. The returned
// *tg.User is converted to the Bot API types.User via convert.User.
//
// Reference: telegram-bot-api/Client.cpp process_get_me_query.
func (c *Client) getMe(ctx context.Context, q *server.Query) (any, error) {
	c.mu.Lock()
	me := c.me
	c.mu.Unlock()
	if me != nil {
		return apitypes.UserForGetMe(me), nil
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}

	// ensureConnected preloads c.me on connect; if that failed, retry once here.
	c.mu.Lock()
	if c.me == nil {
		_ = c.loadMeLocked(ctx)
	}
	me = c.me
	c.mu.Unlock()
	if me == nil {
		return nil, NewError(400, "Bad Request: failed to get bot identity")
	}
	return apitypes.UserForGetMe(me), nil
}

// loadMeLocked fetches the bot's own identity (UsersGetFullUser with
// InputUserSelf) and caches it in c.me. The cached identity is used to enrich
// the From field of every outgoing message (mirrors the reference, which knows
// the bot from auth). The caller MUST hold c.mu. A non-nil error leaves c.me
// untouched and is retried lazily by getMe.
func (c *Client) loadMeLocked(ctx context.Context) error {
	if c.rpc == nil {
		return errors.New("not connected")
	}
	res, err := c.rpc.UsersGetFullUser(ctx, &tg.UsersGetFullUserRequest{
		ID: &tg.InputUserSelf{},
	})
	if err != nil {
		return err
	}
	uf, ok := res.(*tg.UsersUserFull)
	if !ok {
		return fmt.Errorf("unexpected getMe result type %T", res)
	}
	for _, u := range uf.Users {
		if user, ok := u.(*tg.User); ok && user.ID != 0 {
			c.me = convert.User(user)
			return nil
		}
	}
	return errors.New("no bot identity in getMe result")
}
