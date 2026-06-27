package types

// ChatInviteLink represents an invite link for a chat.
// Reference: Client.cpp JsonChatInviteLink lines 1370-1393.
type ChatInviteLink struct {
	InviteLink              string `json:"invite_link"`
	Creator                 User   `json:"creator"`
	CreatesJoinRequest      bool   `json:"creates_join_request"`
	IsPrimary               bool   `json:"is_primary"`
	IsRevoked               bool   `json:"is_revoked"`
	Name                    string `json:"name,omitempty"`
	ExpireDate              int64  `json:"expire_date,omitempty"`
	MemberLimit             int32  `json:"member_limit,omitempty"`
	PendingJoinRequestCount int32  `json:"pending_join_request_count,omitempty"`
	SubscriptionPeriod      int32  `json:"subscription_period,omitempty"`
	SubscriptionPrice       int64  `json:"subscription_price,omitempty"`
}
