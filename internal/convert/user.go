package convert

import (
	"github.com/mtgo-labs/mtgo/tg"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// User converts a raw *tg.User (layer 225, decoded flags) into the Bot API
// types.User. Operates purely on the raw TL struct — no high-level helpers
// (Constitution Principle 2).
//
// Field gotchas:
//   - tg.User.LangCode   → types.User.LanguageCode
//   - tg.User.Bot        → IsBot
//   - !tg.User.BotNochats → CanJoinGroups
//   - tg.User.BotChatHistory → CanReadAllGroupMessages
//   - tg.User.BotInlinePlaceholder != "" → SupportsInlineQueries
func User(u *tg.User) *apitypes.User {
	if u == nil {
		return nil
	}
	return &apitypes.User{
		ID:                        u.ID,
		IsBot:                     u.Bot,
		FirstName:                 u.FirstName,
		LastName:                  u.LastName,
		Username:                  u.Username,
		LanguageCode:              u.LangCode,
		IsPremium:                 u.Premium,
		AddedToAttachmentMenu:     u.AttachMenuEnabled,
		CanJoinGroups:             !u.BotNochats,
		CanReadAllGroupMessages:   u.BotChatHistory,
		SupportsInlineQueries:     u.BotInlinePlaceholder != "",
		CanConnectToBusiness:      u.BotBusiness,
		HasMainWebApp:             u.BotHasMainApp,
		HasTopicsEnabled:          u.BotForumView,
		AllowsUsersToCreateTopics: u.BotForumCanManageTopics,
		CanManageBots:             u.BotCanManageBots,
		SupportsGuestQueries:      u.BotGuestchat,
		SupportsJoinRequestQueries: u.BotGuard,
	}
}
