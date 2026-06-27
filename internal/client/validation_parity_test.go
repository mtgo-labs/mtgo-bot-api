package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// These tests assert client-side validation descriptions match the official
// server word-for-word (Layer 2 of specs/6-error-message-fidelity). They invoke
// the handlers directly; the checked paths return before any Telegram connection
// is required.

func newValidationQuery(method string, args map[string]string) *server.Query {
	q := server.NewQuery()
	q.Method = method
	q.Args = args
	return q
}

func TestSendMessageEmptyText(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendmessage", map[string]string{"chat_id": "123", "text": ""})
	_, err := c.sendMessage(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: message text is empty")
}

func TestEditMessageTextEmptyText(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("editmessagetext", map[string]string{"text": ""})
	_, err := c.editMessageText(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: message text is empty")
}

func TestSendLocationLatitudeEmpty(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendlocation", map[string]string{"chat_id": "123", "latitude": "", "longitude": ""})
	_, err := c.sendLocation(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: latitude is empty")
}

func TestSendLocationLongitudeEmpty(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendlocation", map[string]string{"chat_id": "123", "latitude": "1.0", "longitude": ""})
	_, err := c.sendLocation(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: longitude is empty")
}

func TestSendContactPhoneNumberRequired(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendcontact", map[string]string{"chat_id": "123", "phone_number": "", "first_name": "Bob"})
	_, err := c.sendContact(context.Background(), q)
	assertErr(t, asClientErr(err), 400, `Bad Request: parameter "phone_number" is required`)
}

func TestSendContactFirstNameRequired(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendcontact", map[string]string{"chat_id": "123", "phone_number": "+123", "first_name": ""})
	_, err := c.sendContact(context.Background(), q)
	assertErr(t, asClientErr(err), 400, `Bad Request: parameter "first_name" is required`)
}

func TestSendVenueLatitudeEmpty(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendvenue", map[string]string{"chat_id": "123", "latitude": "", "longitude": "", "title": "", "address": ""})
	_, err := c.sendVenue(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: latitude is empty")
}

func TestSendVenueLongitudeEmpty(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendvenue", map[string]string{"chat_id": "123", "latitude": "1.0", "longitude": "", "title": "", "address": ""})
	_, err := c.sendVenue(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: longitude is empty")
}

// TestSendVenueAllowsEmptyTitleAddress guards the correctness fix: the reference
// process_send_venue_query does NOT validate title/address, so a request with
// valid coordinates but empty title/address must pass validation (reaching the
// connection step, which returns 502 here — NOT a 400 "…are required").
func TestSendVenueAllowsEmptyTitleAddress(t *testing.T) {
	c := NewClient(Params{Dir: t.TempDir()}, "123:abc")
	q := newValidationQuery("sendvenue", map[string]string{"chat_id": "123", "latitude": "1.0", "longitude": "2.0", "title": "", "address": ""})
	_, err := c.sendVenue(context.Background(), q)
	e := asClientErr(err)
	if e.Code == 400 && (e.Description == "Bad Request: chat_id, latitude, longitude, title, and address are required") {
		t.Fatalf("sendVenue wrongly rejected empty title/address: %v", e)
	}
}

// Invoice required fields mirror get_input_message_invoice (Client.cpp:12630-12637),
// validated in order title → description → payload → currency → prices.

func TestSendInvoiceTitleRequired(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendinvoice", map[string]string{"chat_id": "123", "title": "", "description": "d", "payload": "p", "currency": "USD", "prices": "[]"})
	_, err := c.sendInvoice(context.Background(), q)
	assertErr(t, asClientErr(err), 400, `Bad Request: parameter "title" is required`)
}

func TestSendInvoiceCurrencyRequired(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("sendinvoice", map[string]string{"chat_id": "123", "title": "t", "description": "d", "payload": "p", "currency": "", "prices": "[]"})
	_, err := c.sendInvoice(context.Background(), q)
	assertErr(t, asClientErr(err), 400, `Bad Request: parameter "currency" is required`)
}

func TestCreateInvoiceLinkPayloadRequired(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("createinvoicelink", map[string]string{"title": "t", "description": "d", "payload": "", "currency": "USD", "prices": "[]"})
	_, err := c.createInvoiceLink(context.Background(), q)
	assertErr(t, asClientErr(err), 400, `Bad Request: parameter "payload" is required`)
}

// editMessageLiveLocation validates coordinates via get_location (Client.cpp:14268),
// per-field, before connecting.

func TestEditMessageLiveLocationLatitudeEmpty(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("editmessagelivelocation", map[string]string{"chat_id": "123", "message_id": "1", "latitude": "", "longitude": ""})
	_, err := c.editMessageLiveLocation(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: latitude is empty")
}

func TestEditMessageLiveLocationLongitudeEmpty(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := newValidationQuery("editmessagelivelocation", map[string]string{"chat_id": "123", "message_id": "1", "latitude": "1.0", "longitude": ""})
	_, err := c.editMessageLiveLocation(context.Background(), q)
	assertErr(t, asClientErr(err), 400, "Bad Request: longitude is empty")
}

// asClientErr asserts the handler returned a *Error (not a generic error) and
// returns it; fails the test otherwise.
func asClientErr(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{Code: -1, Description: "(non-*Error): " + err.Error()}
}
