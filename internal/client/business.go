package client

import (
	"context"
	"encoding/json"
	"github.com/mtgo-labs/mtgo/tg"
	"math"
	"os"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("getbusinessconnection", (*Client).getBusinessConnection)
	Register("setbusinessaccountname", (*Client).setBusinessAccountName)
	Register("setbusinessaccountbio", (*Client).setBusinessAccountBio)
	Register("setbusinessaccountusername", (*Client).setBusinessAccountUsername)
	Register("readbusinessmessage", (*Client).readBusinessMessage)
	Register("deletebusinessmessages", (*Client).deleteBusinessMessages)
	Register("setbusinessaccountprofilephoto", (*Client).setBusinessAccountProfilePhoto)
	Register("removebusinessaccountprofilephoto", (*Client).removeBusinessAccountProfilePhoto)
	Register("getbusinessaccountstarbalance", (*Client).getBusinessAccountStarBalance)
	Register("setbusinessaccountgiftsettings", (*Client).setBusinessAccountGiftSettings)
	Register("transferbusinessaccountstars", (*Client).transferBusinessAccountStars)
	Register("getbusinessaccountgifts", (*Client).getBusinessAccountGifts)
}

// requireBusinessConn connects and validates the business_connection_id.
// Empty connection_id → "business connection not found" (matches official).
func requireBusinessConn(ctx context.Context, c *Client, q *server.Query) (string, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return "", &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}
	connID := q.Arg("business_connection_id")
	if connID == "" {
		return "", NewError(400, "Bad Request: business connection not found")
	}
	return connID, nil
}

// invokeBusiness runs an MTProto query on behalf of a business connection,
// wrapping it in invokeWithBusinessConnection#dd289f8e. The returned TLObject is
// the inner query's normal result, so callers type-assert it to the same type the
// unwrapped call would return (e.g. tg.UpdatesClass for messages.editMessage).
// Used by methods that accept an optional business_connection_id: when the id is
// empty they call the typed RPC directly; when non-empty they route through here.
func (c *Client) invokeBusiness(ctx context.Context, connID string, query tg.TLObject) (tg.TLObject, error) {
	return c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query:        query,
	})
}

// businessConnID returns the optional business_connection_id parameter, or "".
func businessConnID(q *server.Query) string { return q.Arg("business_connection_id") }

// getBusinessConnection returns info about a business connection.
// Reference: Client.cpp process_get_business_connection_query.
func (c *Client) getBusinessConnection(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	res, err := c.rpc.AccountGetBotBusinessConnection(ctx, &tg.AccountGetBotBusinessConnectionRequest{
		ConnectionID: connID,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	// Extract business connection info from UpdateBotBusinessConnect in the updates.
	conn := &apitypes.BusinessConnection{
		IsEnabled: true,
	}
	// The response is an UpdatesClass. The business connection ID is embedded
	// in the updates but there's no direct UpdateBotBusinessConnection type in tg.
	// Use the connection ID from the request for now.
	conn.ID = connID
	if u, ok := res.(*tg.Updates); ok {
		for _, upd := range u.Updates {
			if bc, ok := upd.(*tg.UpdateBotBusinessConnect); ok && bc.Connection != nil {
				conn.ID = bc.Connection.ConnectionID
				conn.UserID = bc.Connection.UserID
				conn.Date = int64(bc.Connection.Date)
				conn.IsEnabled = !bc.Connection.Disabled
			}
		}
	}
	return conn, nil
}

// setBusinessAccountName sets the first/last name of a business account.
// Reference: Client.cpp process_set_business_account_name_query.
func (c *Client) setBusinessAccountName(ctx context.Context, q *server.Query) (any, error) {
	firstName := q.Arg("first_name")
	if firstName == "" {
		return nil, NewError(400, "Bad Request: parameter \"first_name\" is required")
	}
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	profReq := &tg.AccountUpdateProfileRequest{
		FirstName: firstName,
		LastName:  q.Arg("last_name"),
	}
	profReq.SetFlags()
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query:        profReq,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setBusinessAccountBio sets the bio of a business account.
// Reference: Client.cpp process_set_business_account_bio_query.
func (c *Client) setBusinessAccountBio(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	profReq := &tg.AccountUpdateProfileRequest{
		About: q.Arg("bio"),
	}
	profReq.SetFlags()
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query:        profReq,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setBusinessAccountUsername sets the username of a business account.
// Reference: Client.cpp process_set_business_account_username_query.
func (c *Client) setBusinessAccountUsername(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.AccountUpdateUsernameRequest{
			Username: q.Arg("username"),
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// readBusinessMessage marks a business message as read.
// Reference: Client.cpp process_read_business_message_query.
func (c *Client) readBusinessMessage(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat identifier is empty")
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	_ = peer // peer used later
	msgID, err := q.ArgInt64("message_id")
	if err != nil {
		return nil, NewError(400, "Bad Request: parameter \"message_id\" is required")
	}
	if msgID > math.MaxInt32 {
		return nil, NewError(400, "Bad Request: message_id is too large")
	}
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.MessagesReadHistoryRequest{
			Peer:  peer,
			MaxID: int32(msgID),
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteBusinessMessages deletes messages on behalf of a business account.
// Reference: Client.cpp process_delete_business_messages_query.
func (c *Client) deleteBusinessMessages(ctx context.Context, q *server.Query) (any, error) {
	ids := parseMessageIDs(q)
	if len(ids) == 0 {
		return nil, NewError(400, "Bad Request: message identifiers are not specified")
	}
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.MessagesDeleteMessagesRequest{
			ID:     ids,
			Revoke: true,
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setBusinessAccountProfilePhoto sets the profile photo of a business account.
// Reference: Client.cpp process_set_business_account_profile_photo_query.
func (c *Client) setBusinessAccountProfilePhoto(ctx context.Context, q *server.Query) (any, error) {
	photoFile, ok := q.File("photo")
	if !ok {
		return nil, NewError(400, "Bad Request: photo isn't specified")
	}
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(photoFile.TempPath)
	if err != nil {
		return nil, NewError(400, "Bad Request: failed to read uploaded file")
	}
	defer func() { _ = f.Close() }()

	fileID, err := generateFileID()
	if err != nil {
		return nil, NewError(500, "Internal Server Error: file ID generation failed")
	}
	inputFile, err := c.uploadFile(ctx, fileID, photoFile.FileName, photoFile.Size, f)
	if err != nil {
		return nil, rpcError(err)
	}

	photoReq := &tg.PhotosUploadProfilePhotoRequest{
		File: inputFile,
	}
	// is_public → TL photos.uploadProfilePhoto fallback (flag 3). Traced via TDLib:
	// setBusinessAccountProfilePhoto is_public → user_manager is_fallback →
	// photos_uploadProfilePhoto fallback. (Bot API "public" photo == MTProto "fallback"
	// photo: the one shown when the main photo is hidden by privacy settings.)
	photoReq.Fallback = q.ArgBool("is_public")
	photoReq.SetFlags()

	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query:        photoReq,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// removeBusinessAccountProfilePhoto removes the profile photo of a business account.
// Reference: Client.cpp process_remove_business_account_profile_photo_query.
// Uses PhotosDeletePhotos since AccountDeleteProfilePhoto is not generated.
func (c *Client) removeBusinessAccountProfilePhoto(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	// Mirrors TDLib DeleteBusinessProfilePhotoQuery: removeBusinessAccountProfilePhoto →
	// setBusinessAccountProfilePhoto(photo=null, is_public) → photos.updateProfilePhoto with
	// inputPhotoEmpty + fallback=is_public, wrapped in InvokeWithBusinessConnection. The Bot API
	// method has no "photo" param — it clears the current public/fallback photo.
	req := &tg.PhotosUpdateProfilePhotoRequest{
		ID:       &tg.InputPhotoEmpty{},
		Fallback: q.ArgBool("is_public"),
	}
	req.SetFlags()
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query:        req,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getBusinessAccountStarBalance returns the Stars balance of a business account.
// Reference: Client.cpp process_get_business_account_star_balance_query.
// Uses PaymentsGetStarsStatus wrapped in InvokeWithBusinessConnection.
// Returns {"amount": N, "nanostar_amount": N} (nanostar_amount omitted when 0).
func (c *Client) getBusinessAccountStarBalance(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	result, err := c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.PaymentsGetStarsStatusRequest{
			Peer: &tg.InputPeerSelf{},
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	status, ok := result.(*tg.PaymentsStarsStatus)
	if !ok || status.Balance == nil {
		return map[string]any{"amount": 0}, nil
	}
	if amt, ok := status.Balance.(*tg.StarsAmount); ok {
		m := map[string]any{"amount": amt.Amount}
		if amt.Nanos != 0 {
			m["nanostar_amount"] = amt.Nanos
		}
		return m, nil
	}
	return map[string]any{"amount": 0}, nil
}

// setBusinessAccountGiftSettings sets gift settings for a business account.
// Reference: Client.cpp:15279, TDLib: account.setGlobalPrivacySettings via InvokeWithBusinessConnection.
// Params: business_connection_id, show_gift_button, accepted_gift_types (JSON).
func (c *Client) setBusinessAccountGiftSettings(ctx context.Context, q *server.Query) (any, error) {
	// Validate accepted_gift_types BEFORE connection (matches official order).
	if raw := q.Arg("accepted_gift_types"); raw == "" {
		return nil, NewError(400, "Bad Request: accepted gift types aren't specified")
	} else {
		var at acceptedGiftTypes
		if err := json.Unmarshal([]byte(raw), &at); err != nil {
			return nil, NewError(400, "Bad Request: can't parse accepted_gift_types JSON object")
		}
	}
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	settings := &tg.GlobalPrivacySettings{
		DisplayGiftsButton: q.ArgBool("show_gift_button"),
	}
	if raw := q.Arg("accepted_gift_types"); raw != "" {
		var at acceptedGiftTypes
		if err := json.Unmarshal([]byte(raw), &at); err == nil {
			settings.DisallowedGifts = &tg.DisallowedGiftsSettings{
				DisallowUnlimitedStargifts:    !at.UnlimitedGifts,
				DisallowLimitedStargifts:      !at.LimitedGifts,
				DisallowUniqueStargifts:       !at.UniqueGifts,
				DisallowPremiumGifts:          !at.PremiumSubscription,
				DisallowStargiftsFromChannels: !at.GiftsFromChannels,
			}
			settings.DisallowedGifts.SetFlags()
			settings.Flags.Set(6)
		}
	}
	settings.SetFlags()
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.AccountSetGlobalPrivacySettingsRequest{
			Settings: settings,
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// acceptedGiftTypes mirrors the Bot API accepted_gift_types JSON object.
// Reference: Client.cpp:12986 get_accepted_gift_types.
type acceptedGiftTypes struct {
	UnlimitedGifts      bool `json:"unlimited_gifts"`
	LimitedGifts        bool `json:"limited_gifts"`
	UniqueGifts         bool `json:"unique_gifts"`
	PremiumSubscription bool `json:"premium_subscription"`
	GiftsFromChannels   bool `json:"gifts_from_channels"`
}

// transferBusinessAccountStars transfers Stars from a business account to the bot.
// Reference: Client.cpp:15304, TDLib: two-step payments.getPaymentForm + payments.sendStarsForm.
// Params: business_connection_id, star_count.
func (c *Client) transferBusinessAccountStars(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	starCount, err := q.ArgInt64("star_count")
	if err != nil || starCount <= 0 {
		return nil, NewError(400, "Bad Request: star_count must be a positive integer")
	}

	// Step 1: Get payment form.
	invoice := &tg.InputInvoiceBusinessBotTransferStars{
		Bot:   &tg.InputUserSelf{},
		Stars: starCount,
	}
	formResult, err := c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.PaymentsGetPaymentFormRequest{
			Invoice: invoice,
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	form, ok := formResult.(*tg.PaymentsPaymentFormStars)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected payment form type")
	}

	// Step 2: Send stars form.
	_, err = c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query: &tg.PaymentsSendStarsFormRequest{
			FormID:  form.FormID,
			Invoice: invoice,
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getBusinessAccountGifts returns gifts received by a business account.
// Reference: Client.cpp:15316, TDLib: payments.getSavedStarGifts via InvokeWithBusinessConnection.
// Params: business_connection_id, offset, limit, exclude_* filters.
func (c *Client) getBusinessAccountGifts(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	req := &tg.PaymentsGetSavedStarGiftsRequest{
		Peer:   &tg.InputPeerSelf{},
		Offset: q.Arg("offset"),
		Limit:  100,
	}
	if v, e := q.ArgInt64("limit"); e == nil && v > 0 {
		req.Limit = int32(v)
	}
	req.ExcludeUnsaved = q.ArgBool("exclude_unsaved")
	req.ExcludeSaved = q.ArgBool("exclude_saved")
	req.ExcludeUnlimited = q.ArgBool("exclude_unlimited")
	req.ExcludeUnique = q.ArgBool("exclude_unique")
	req.SortByValue = q.ArgBool("sort_by_price")
	req.ExcludeUpgradable = q.ArgBool("exclude_limited_upgradable")
	req.ExcludeUnupgradable = q.ArgBool("exclude_limited_non_upgradable")
	req.ExcludeHosted = q.ArgBool("exclude_from_blockchain")
	// exclude_limited is a composite flag that sets both upgradable and non-upgradable.
	if q.ArgBool("exclude_limited") {
		req.ExcludeUpgradable = true
		req.ExcludeUnupgradable = true
	}
	req.SetFlags()

	result, err := c.rpc.InvokeWithBusinessConnection(ctx, &tg.InvokeWithBusinessConnectionRequest{
		ConnectionID: connID,
		Query:        req,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return result, nil
}
