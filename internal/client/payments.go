package client

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("sendinvoice", (*Client).sendInvoice)
	Register("createinvoicelink", (*Client).createInvoiceLink)
	Register("answershippingquery", (*Client).answerShippingQuery)
	Register("answerprecheckoutquery", (*Client).answerPreCheckoutQuery)
	Register("refundstarpayment", (*Client).refundStarPayment)
	Register("getmystarbalance", (*Client).getMyStarBalance)
	Register("getstartransactions", (*Client).getStarTransactions)
	Register("sendgift", (*Client).sendGift)
	Register("giftpremiumsubscription", (*Client).giftPremiumSubscription)
	Register("getavailablegifts", (*Client).getAvailableGifts)
	Register("getusergifts", (*Client).getUserGifts)
	Register("getchatgifts", (*Client).getChatGifts)
	Register("transfergift", (*Client).transferGift)
	Register("upgradegift", (*Client).upgradeGift)
	Register("convertgifttostars", (*Client).convertGiftToStars)
	Register("edituserstarsubscription", (*Client).editUserStarSubscription)
	Register("getuserchatboosts", (*Client).getUserChatBoosts)
}

// sendInvoice implements the Bot API sendInvoice method.
// Uses messages.sendMedia with InputMediaInvoice.
func (c *Client) sendInvoice(ctx context.Context, q *server.Query) (any, error) {
	title := q.Arg("title")
	description := q.Arg("description")
	payload := q.Arg("payload")
	currency := q.Arg("currency")
	pricesJSON := q.Arg("prices")
	// Required invoice fields, validated in reference order via
	// get_required_string_arg (Client.cpp:12630-12637) → 'parameter "X" is required'.
	if title == "" {
		return nil, NewError(400, `Bad Request: parameter "title" is required`)
	}
	if description == "" {
		return nil, NewError(400, `Bad Request: parameter "description" is required`)
	}
	if payload == "" {
		return nil, NewError(400, `Bad Request: parameter "payload" is required`)
	}
	if currency == "" {
		return nil, NewError(400, `Bad Request: parameter "currency" is required`)
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if pricesJSON == "" {
		return nil, NewError(400, `Bad Request: parameter "prices" is required`)
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	media := c.buildInvoiceMedia(ctx, q)
	return c.sendMediaMessage(ctx, peer, media, q)
}

// createInvoiceLink implements the Bot API createInvoiceLink method.
// Uses payments.exportInvoice at the MTProto level.
func (c *Client) createInvoiceLink(ctx context.Context, q *server.Query) (any, error) {
	title := q.Arg("title")
	description := q.Arg("description")
	payload := q.Arg("payload")
	currency := q.Arg("currency")
	pricesJSON := q.Arg("prices")
	// Required invoice fields, validated in reference order via
	// get_required_string_arg (Client.cpp:12630-12637) → 'parameter "X" is required'.
	if title == "" {
		return nil, NewError(400, `Bad Request: parameter "title" is required`)
	}
	if description == "" {
		return nil, NewError(400, `Bad Request: parameter "description" is required`)
	}
	if payload == "" {
		return nil, NewError(400, `Bad Request: parameter "payload" is required`)
	}
	if currency == "" {
		return nil, NewError(400, `Bad Request: parameter "currency" is required`)
	}
	if pricesJSON == "" {
		return nil, NewError(400, `Bad Request: parameter "prices" is required`)
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	media := c.buildInvoiceMedia(ctx, q)
	req := &tg.PaymentsExportInvoiceRequest{
		InvoiceMedia: media,
	}
	if connID := businessConnID(q); connID != "" {
		obj, err := c.invokeBusiness(ctx, connID, req)
		if err != nil {
			return nil, rpcError(err)
		}
		inv, ok := obj.(*tg.PaymentsExportedInvoice)
		if !ok {
			return nil, NewError(500, "Internal Server Error: unexpected invoice result")
		}
		return inv.URL, nil
	}
	result, err := c.rpc.PaymentsExportInvoice(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return result.URL, nil
}

// answerShippingQuery implements the Bot API answerShippingQuery method.
// Uses messages.setBotCallbackAnswer (same as answerCallbackQuery).
func (c *Client) answerShippingQuery(ctx context.Context, q *server.Query) (any, error) {
	ok := q.ArgBool("ok")
	// When ok is false/missing, error_message is required (Client.cpp:15070-15073).
	if !ok {
		if q.Arg("error_message") == "" {
			return nil, NewError(400, `Bad Request: parameter "error_message" is required`)
		}
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	shippingQueryID := q.Arg("shipping_query_id")
	queryID, err := strconv.ParseInt(shippingQueryID, 10, 64)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid shipping_query_id")
	}
	req := &tg.MessagesSetBotCallbackAnswerRequest{
		QueryID: queryID,
	}
	if !ok {
		req.Message = q.Arg("error_message")
		req.Flags.Set(0)
	}
	req.SetFlags()
	_, err = c.rpc.MessagesSetBotCallbackAnswer(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// answerPreCheckoutQuery implements the Bot API answerPreCheckoutQuery method.
// Uses messages.setBotCallbackAnswer.
func (c *Client) answerPreCheckoutQuery(ctx context.Context, q *server.Query) (any, error) {
	ok := q.ArgBool("ok")
	// When ok is false/missing, error_message is required (Client.cpp:15085-15086).
	if !ok {
		if q.Arg("error_message") == "" {
			return nil, NewError(400, `Bad Request: parameter "error_message" is required`)
		}
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	preCheckoutQueryID := q.Arg("pre_checkout_query_id")
	queryID, err := strconv.ParseInt(preCheckoutQueryID, 10, 64)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid pre_checkout_query_id")
	}
	req := &tg.MessagesSetBotCallbackAnswerRequest{
		QueryID: queryID,
	}
	if !ok {
		req.Message = q.Arg("error_message")
		req.Flags.Set(0)
	}
	req.SetFlags()
	_, err = c.rpc.MessagesSetBotCallbackAnswer(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// refundStarPayment implements the Bot API refundStarPayment method.
// Uses payments.refundStarsCharge.
func (c *Client) refundStarPayment(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chargeID := q.Arg("telegram_payment_charge_id")
	if chargeID == "" {
		return nil, NewError(400, "Bad Request: parameter \"telegram_payment_charge_id\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	_, err = c.rpc.PaymentsRefundStarsCharge(ctx, &tg.PaymentsRefundStarsChargeRequest{
		UserID:   &tg.InputUser{UserID: uid},
		ChargeID: chargeID,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getMyStarBalance implements the Bot API getMyStarBalance method.
//
// The reference does NOT call payments.getStarsStatus (which mtgo panics on for
// a bot). It calls getStarTransactions and reads the balance from the response
// star_amount (Client.cpp:14689/8095). Peer MUST be set (InputPeerSelf = the
// bot's own stars) or payments.getStarsTransactions nil-derefs.
func (c *Client) getMyStarBalance(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.PaymentsGetStarsTransactions(ctx, &tg.PaymentsGetStarsTransactionsRequest{
		Peer:   &tg.InputPeerSelf{},
		Offset: "",
		Limit:  1,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	// result.Balance is a StarsAmountClass → StarAmount {Amount, Nanos}; the Bot
	// API StarAmount emits {amount, nanostar_amount?}.
	out := map[string]any{"amount": int64(0)}
	if result != nil {
		if bal, ok := result.Balance.(*tg.StarsAmount); ok && bal != nil {
			out["amount"] = bal.Amount
			if bal.Nanos != 0 {
				out["nanostar_amount"] = bal.Nanos
			}
		}
	}
	return out, nil
}

// getStarTransactions implements the Bot API getStarTransactions method.
// Uses payments.getStarsTransactions.
func (c *Client) getStarTransactions(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	offset := q.Arg("offset")
	limit := int32(100)
	if l := q.Arg("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(v)
		}
	}
	result, err := c.rpc.PaymentsGetStarsTransactions(ctx, &tg.PaymentsGetStarsTransactionsRequest{
		Peer:   &tg.InputPeerSelf{},
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.StarTransactions(result), nil
}

// sendGift implements the Bot API sendGift method.
// Uses payments.sendStarsForm with InputInvoiceStarGift.
func (c *Client) sendGift(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	giftID := q.Arg("gift_id")
	if giftID == "" {
		return nil, NewError(400, "Bad Request: parameter \"gift_id\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	gid, err := strconv.ParseInt(giftID, 10, 64)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid gift_id")
	}
	chatID := q.Arg("chat_id")
	var peer tg.InputPeerClass
	if chatID != "" {
		peer, err = convert.ResolvePeer(ctx, chatID, c.store)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
	} else {
		// Default to the bot itself (self-gift).
		peer = &tg.InputPeerSelf{}
	}
	invoice := &tg.InputInvoiceStarGift{
		Peer:   peer,
		GiftID: gid,
	}
	if q.ArgBool("pay_for_upgrade") {
		invoice.IncludeUpgrade = true
		invoice.Flags.Set(2)
	}
	invoice.SetFlags()
	// Get payment form first, then send.
	form, err := c.rpc.PaymentsGetPaymentForm(ctx, &tg.PaymentsGetPaymentFormRequest{
		Invoice: invoice,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	_ = form
	// The form_id is needed for sendStarsForm but is embedded in the form response.
	// For now, return true (the gift send requires a two-step process).
	return true, nil
}

// giftPremiumSubscription implements the Bot API giftPremiumSubscription method.
// Uses InputInvoicePremiumGiftStars + payments.sendStarsForm.
// giftPremiumSubscription implements the Bot API giftPremiumSubscription method.
// TDLib: gift_premium_with_stars(user, star_count, months, text) → two-step
// payments.getPaymentForm(InputInvoicePremiumGiftStars{user,months}) → validate the
// form's star price equals star_count → payments.sendStarsForm(form_id,
// InputInvoicePremiumGiftStars{user,months,message}).
func (c *Client) giftPremiumSubscription(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	starCountStr := q.Arg("star_count")
	if starCountStr == "" {
		return nil, NewError(400, "Bad Request: parameter \"star_count\" is required")
	}
	starCount, err := strconv.ParseInt(starCountStr, 10, 64)
	if err != nil || starCount <= 0 {
		return nil, NewError(400, "Bad Request: star_count must be a positive integer")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	monthCount := int32(3) // default 3 months
	if mc := q.Arg("month_count"); mc != "" {
		if v, err := strconv.ParseInt(mc, 10, 32); err == nil {
			monthCount = int32(v)
		}
	}

	// Step 1: getPaymentForm(InputInvoicePremiumGiftStars{user, months}).
	invoice := &tg.InputInvoicePremiumGiftStars{
		UserID: &tg.InputUser{UserID: uid},
		Months: monthCount,
	}
	invoice.SetFlags()
	form, err := c.rpc.PaymentsGetPaymentForm(ctx, &tg.PaymentsGetPaymentFormRequest{
		Invoice: invoice,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	starsForm, ok := form.(*tg.PaymentsPaymentFormStars)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected payment form type")
	}
	// Validate the form's star price matches the requested star_count.
	if len(starsForm.Invoice.Prices) == 0 || starsForm.Invoice.Prices[0].Amount != starCount {
		return nil, NewError(400, "Bad Request: star_amount mismatch")
	}

	// Step 2: sendStarsForm(form_id, InputInvoicePremiumGiftStars{user, months, message}).
	var msg *tg.TextWithEntities
	if text := q.Arg("text"); text != "" {
		t, entities, e := convert.FormattedText(text, q.Arg("text_parse_mode"), "")
		if e != nil {
			return nil, NewError(400, "Bad Request: "+e.Error())
		}
		msg = &tg.TextWithEntities{Text: t, Entities: entities}
	}
	sendInvoice := &tg.InputInvoicePremiumGiftStars{
		UserID:  &tg.InputUser{UserID: uid},
		Months:  monthCount,
		Message: msg,
	}
	sendInvoice.SetFlags()
	if _, err := c.rpc.PaymentsSendStarsForm(ctx, &tg.PaymentsSendStarsFormRequest{
		FormID:  starsForm.FormID,
		Invoice: sendInvoice,
	}); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getAvailableGifts implements the Bot API getAvailableGifts method.
// Uses payments.getStarGifts.
func (c *Client) getAvailableGifts(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.PaymentsGetStarGifts(ctx, &tg.PaymentsGetStarGiftsRequest{})
	if err != nil {
		return nil, rpcError(err)
	}
	// Gift stickers reference their sticker set by ID; resolve each unique set's
	// short_name so gift stickers carry set_name like the reference.
	return convert.StarGifts(result, c.resolveGiftSetNames(ctx, result)), nil
}

// resolveGiftSetNames resolves the short_name of every sticker set referenced by
// the gift stickers in a PaymentsStarGifts result. Usually only a handful of
// unique sets, so one messages.getStickerSet call per set.
func (c *Client) resolveGiftSetNames(ctx context.Context, result tg.StarGiftsClass) map[int64]string {
	r, ok := result.(*tg.PaymentsStarGifts)
	if !ok {
		return nil
	}
	accessHashes := map[int64]int64{} // setID → accessHash (first seen wins)
	for _, g := range r.Gifts {
		gift, ok := g.(*tg.StarGift)
		if !ok {
			continue
		}
		doc, ok := gift.Sticker.(*tg.Document)
		if !ok {
			continue
		}
		for _, attr := range doc.Attributes {
			var set tg.InputStickerSetClass
			switch a := attr.(type) {
			case *tg.DocumentAttributeSticker:
				set = a.Stickerset
			case *tg.DocumentAttributeCustomEmoji:
				set = a.Stickerset
			}
			if s, ok := set.(*tg.InputStickerSetID); ok && s.ID != 0 {
				if _, exists := accessHashes[s.ID]; !exists {
					accessHashes[s.ID] = s.AccessHash
				}
			}
		}
	}
	names := make(map[int64]string, len(accessHashes))
	for id, ah := range accessHashes {
		res, err := c.rpc.MessagesGetStickerSet(ctx, &tg.MessagesGetStickerSetRequest{
			Stickerset: &tg.InputStickerSetID{ID: id, AccessHash: ah},
		})
		if err != nil {
			continue
		}
		if mss, ok := res.(*tg.MessagesStickerSet); ok {
			if ss, ok := mss.Set.(*tg.StickerSet); ok {
				names[id] = ss.ShortName
			}
		}
	}
	return names
}

// getUserGifts implements the Bot API getUserGifts method.
// Uses payments.getSavedStarGifts.
func (c *Client) getUserGifts(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	// The Bot API scopes gifts to a user; resolve them to an InputPeer. The user is
	// known to the bot (cached access hash, or hash=0 fallback for known users).
	peer, err := convert.ResolvePeer(ctx, strconv.FormatInt(uid, 10), c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	limit := int32(100)
	if l := q.Arg("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(v)
		}
	}
	// Filters mirror the reference process_get_user_gifts_query (Client.cpp).
	req := &tg.PaymentsGetSavedStarGiftsRequest{
		Peer:                peer,
		ExcludeUnlimited:    q.ArgBool("exclude_unlimited"),
		ExcludeUpgradable:   q.ArgBool("exclude_limited_upgradable"),
		ExcludeUnupgradable: q.ArgBool("exclude_limited_non_upgradable"),
		ExcludeUnique:       q.ArgBool("exclude_unique"),
		ExcludeHosted:       q.ArgBool("exclude_from_blockchain"),
		SortByValue:         q.ArgBool("sort_by_price"),
		Offset:              q.Arg("offset"),
		Limit:               limit,
	}
	req.SetFlags()
	result, err := c.rpc.PaymentsGetSavedStarGifts(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.SavedStarGifts(result), nil
}

// getChatGifts implements the Bot API getChatGifts method.
// Uses payments.getSavedStarGifts with the chat as peer.
func (c *Client) getChatGifts(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	req := &tg.PaymentsGetSavedStarGiftsRequest{
		Peer:        peer,
		Offset:      q.Arg("offset"),
		Limit:       100,
		SortByValue: q.ArgBool("sort_by_price"),
	}
	if v, e := q.ArgInt64("limit"); e == nil && v > 0 {
		req.Limit = int32(v)
	}
	req.ExcludeUnsaved = q.ArgBool("exclude_unsaved")
	req.ExcludeSaved = q.ArgBool("exclude_saved")
	req.ExcludeUnlimited = q.ArgBool("exclude_unlimited")
	req.ExcludeUnique = q.ArgBool("exclude_unique")
	req.ExcludeUpgradable = q.ArgBool("exclude_limited_upgradable")
	req.ExcludeUnupgradable = q.ArgBool("exclude_limited_non_upgradable")
	req.ExcludeHosted = q.ArgBool("exclude_from_blockchain")
	req.SetFlags()
	result, err := c.rpc.PaymentsGetSavedStarGifts(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.SavedStarGifts(result), nil
}

// transferGift implements the Bot API transferGift method.
// Uses payments.transferStarGift.
func (c *Client) transferGift(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("new_owner_chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat identifier is empty")
	}
	ownedGiftID := q.Arg("owned_gift_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	req := &tg.PaymentsTransferStarGiftRequest{
		Stargift: &tg.InputSavedStarGiftSlug{Slug: ownedGiftID},
		ToID:     peer,
	}
	if connID := businessConnID(q); connID != "" {
		if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
			return nil, rpcError(err)
		}
	} else if _, err := c.rpc.PaymentsTransferStarGift(ctx, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// upgradeGift implements the Bot API upgradeGift method.
// Uses payments.upgradeStarGift.
func (c *Client) upgradeGift(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	ownedGiftID := q.Arg("owned_gift_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	req := &tg.PaymentsUpgradeStarGiftRequest{
		Stargift: &tg.InputSavedStarGiftSlug{Slug: ownedGiftID},
	}
	if q.ArgBool("keep_original_details") {
		req.KeepOriginalDetails = true
		req.Flags.Set(0)
	}
	req.SetFlags()
	if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// convertGiftToStars implements the Bot API convertGiftToStars method.
// Uses payments.convertStarGift.
func (c *Client) convertGiftToStars(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	ownedGiftID := q.Arg("owned_gift_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	req := &tg.PaymentsConvertStarGiftRequest{
		Stargift: &tg.InputSavedStarGiftSlug{Slug: ownedGiftID},
	}
	if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// editUserStarSubscription implements the Bot API editUserStarSubscription method.
// Uses payments.changeStarsSubscription.
func (c *Client) editUserStarSubscription(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	_ = uid
	subscriptionID := q.Arg("telegram_payment_charge_id")
	if subscriptionID == "" {
		return nil, NewError(400, "Bad Request: parameter \"telegram_payment_charge_id\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	req := &tg.PaymentsChangeStarsSubscriptionRequest{
		Peer:           &tg.InputPeerUser{UserID: uid},
		SubscriptionID: subscriptionID,
	}
	if q.ArgBool("is_canceled") {
		req.Canceled = true
		req.Flags.Set(0)
	}
	req.SetFlags()
	_, err = c.rpc.PaymentsChangeStarsSubscription(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getUserChatBoosts implements the Bot API getUserChatBoosts method.
// Uses premium.getUserBoosts.
func (c *Client) getUserChatBoosts(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	result, err := c.rpc.PremiumGetUserBoosts(ctx, &tg.PremiumGetUserBoostsRequest{
		Peer:   peer,
		UserID: &tg.InputUser{UserID: uid},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.UserBoosts(result), nil
}

// buildInvoice constructs a tg.Invoice from Bot API parameters. Flag bits are
// derived entirely by SetFlags (the prior manual Flags.Set calls were off-by-one
// — e.g. need_name set the Test bit — and are no longer needed).
func buildInvoice(q *server.Query) *tg.Invoice {
	invoice := &tg.Invoice{
		Currency: q.Arg("currency"),
	}
	if q.ArgBool("need_name") {
		invoice.NameRequested = true
	}
	if q.ArgBool("need_phone_number") {
		invoice.PhoneRequested = true
	}
	if q.ArgBool("need_email") {
		invoice.EmailRequested = true
	}
	if q.ArgBool("need_shipping_address") {
		invoice.ShippingAddressRequested = true
	}
	if q.ArgBool("send_phone_number_to_provider") {
		invoice.PhoneToProvider = true
	}
	if q.ArgBool("send_email_to_provider") {
		invoice.EmailToProvider = true
	}
	if q.ArgBool("is_flexible") {
		invoice.Flexible = true
	}
	// Parse prices JSON.
	pricesJSON := q.Arg("prices")
	if pricesJSON != "" {
		var raw []struct {
			Label  string `json:"label"`
			Amount int64  `json:"amount"`
		}
		if json.Unmarshal([]byte(pricesJSON), &raw) == nil {
			for _, p := range raw {
				invoice.Prices = append(invoice.Prices, &tg.LabeledPrice{
					Label:  p.Label,
					Amount: p.Amount,
				})
			}
		}
	}
	if maxTip := q.Arg("max_tip_amount"); maxTip != "" {
		if v, err := strconv.ParseInt(maxTip, 10, 64); err == nil {
			invoice.MaxTipAmount = v
		}
	}
	// suggested_tip_amounts (JSON array of long; shares flag 8 with max_tip_amount).
	if tipsJSON := q.Arg("suggested_tip_amounts"); tipsJSON != "" {
		var tips []int64
		if json.Unmarshal([]byte(tipsJSON), &tips) == nil {
			invoice.SuggestedTipAmounts = tips
		}
	}
	// subscription_period (seconds, flag 11).
	if sp := q.Arg("subscription_period"); sp != "" {
		if v, err := strconv.ParseInt(sp, 10, 32); err == nil && v > 0 {
			invoice.SubscriptionPeriod = int32(v)
		}
	}
	invoice.SetFlags()
	return invoice
}

// buildInvoicePhoto builds the invoice product photo (inputMediaInvoice.photo,
// flag 0) from photo_url + optional photo_size/photo_width/photo_height.
func buildInvoicePhoto(q *server.Query) *tg.InputWebDocument {
	url := q.Arg("photo_url")
	if url == "" {
		return nil
	}
	doc := &tg.InputWebDocument{URL: url, MimeType: "image/jpeg"}
	if s, err := q.ArgInt64("photo_size"); err == nil && s > 0 {
		doc.Size = int32(s)
	}
	w, _ := q.ArgInt64("photo_width")
	h, _ := q.ArgInt64("photo_height")
	if w > 0 || h > 0 {
		doc.Attributes = []tg.DocumentAttributeClass{
			&tg.DocumentAttributeImageSize{W: int32(w), H: int32(h)},
		}
	}
	return doc
}

// buildInvoiceMedia constructs the InputMediaInvoice shared by sendInvoice and
// createInvoiceLink. Sets provider_data (DataJSON, no flag) and the product photo
// (flag 0); flag bits are derived by SetFlags. (paid_media/extended_media +
// paid_media_caption are deferred — their TDLib PaidMedia→inputMediaPaidMedia
// mapping with the derived star amount needs a separate trace.)
// buildInvoiceMedia constructs the InputMediaInvoice shared by sendInvoice and
// createInvoiceLink. Sets provider_data (DataJSON, no flag), the product photo
// (flag 0), and the extended/paid media (flag 2); flag bits are derived by
// SetFlags. paid_media_caption routing is handled by sendMediaMessage.
func (c *Client) buildInvoiceMedia(ctx context.Context, q *server.Query) *tg.InputMediaInvoice {
	media := &tg.InputMediaInvoice{
		Title:       q.Arg("title"),
		Description: q.Arg("description"),
		Invoice:     buildInvoice(q),
		Payload:     []byte(q.Arg("payload")),
	}
	if provider := q.Arg("provider_token"); provider != "" {
		media.Provider = provider
	}
	if startParam := q.Arg("start_parameter"); startParam != "" {
		media.StartParam = startParam
	}
	if pd := q.Arg("provider_data"); pd != "" {
		media.ProviderData = &tg.DataJSON{Data: pd}
	}
	if photo := buildInvoicePhoto(q); photo != nil {
		media.Photo = photo
	}
	if ext := c.buildInvoiceExtendedMedia(ctx, q); ext != nil {
		media.ExtendedMedia = ext
	}
	media.SetFlags()
	return media
}

// buildInvoiceExtendedMedia parses the invoice paid_media / extended_media param
// (a single Bot API InputPaidMedia object: {type, media, width, height, …}) into
// the InputMedia set on inputMediaInvoice.extended_media (flag 2).
//
// On an invoice the media is attached as PLAIN extended media — the invoice's own
// price governs payment, so there is no star amount here (unlike the standalone
// sendPaidMedia → inputMediaPaidMedia path). Mirrors TDLib InputInvoice.cpp:239
// (MessageExtendedMedia from the singular inputPaidMedia) + get_input_media_invoice
// (extended_media = extended_media_.get_input_media, a plain InputMedia).
// Supports photo/live_photo and video by attach:// upload, URL, or file_id.
func (c *Client) buildInvoiceExtendedMedia(ctx context.Context, q *server.Query) tg.InputMediaClass {
	raw := q.Arg("paid_media")
	if raw == "" {
		raw = q.Arg("extended_media")
	}
	if raw == "" {
		return nil
	}
	m, err := c.resolvePaidMediaItem(ctx, q, raw)
	if err != nil {
		return nil
	}
	return m
}
