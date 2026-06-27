package convert

import (
	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// ChatInviteLinkFromExported converts a tg.ChatInviteExported (the result of
// messages.exportChatInvite / editExportedChatInvite) into the Bot API
// ChatInviteLink type.
//
// The creator User is built from AdminID only; full user details require a
// separate users.getFullUser call which the reference server resolves via
// TDLib's user cache. If the peer cache holds the admin, callers may enrich
// the returned User afterwards.
func ChatInviteLinkFromExported(e *tg.ChatInviteExported) *types.ChatInviteLink {
	if e == nil {
		return nil
	}
	link := &types.ChatInviteLink{
		InviteLink:  e.Link,
		Creator:     types.User{ID: e.AdminID},
		IsPrimary:   e.Permanent,
		IsRevoked:   e.Revoked,
		Name:        e.Title,
		ExpireDate:  int64(e.ExpireDate),
		MemberLimit: e.UsageLimit,
		PendingJoinRequestCount: e.Requested,
		CreatesJoinRequest: e.RequestNeeded,
	}
	if e.SubscriptionPricing != nil {
		link.SubscriptionPeriod = e.SubscriptionPricing.Period
		link.SubscriptionPrice = e.SubscriptionPricing.Amount
	}
	return link
}
