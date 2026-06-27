package client

import (
	"context"

	"github.com/mtgo-labs/mtgo/tg"
)

// fakeRPC is a test double for rpcInvoker. By default every method returns
// its zero value with no error. Override specific function fields to control
// behavior in a test. This avoids the need for a real Telegram connection.
type fakeRPC struct {
	// Override fields — set these in tests to control specific method behavior.
	// Unset methods return zero values.

	AccountGetBotBusinessConnectionFn       func(ctx context.Context, req *tg.AccountGetBotBusinessConnectionRequest) (tg.UpdatesClass, error)
	AuthLogOutFn                            func(ctx context.Context) (*tg.AuthLoggedOut, error)
	BotsAnswerWebhookJSONQueryFn            func(ctx context.Context, req *tg.BotsAnswerWebhookJSONQueryRequest) (bool, error)
	BotsEditAccessSettingsFn                func(ctx context.Context, req *tg.BotsEditAccessSettingsRequest) (bool, error)
	BotsExportBotTokenFn                    func(ctx context.Context, req *tg.BotsExportBotTokenRequest) (*tg.BotsExportedBotToken, error)
	BotsGetAccessSettingsFn                 func(ctx context.Context, req *tg.BotsGetAccessSettingsRequest) (*tg.BotsAccessSettings, error)
	BotsGetBotInfoFn                        func(ctx context.Context, req *tg.BotsGetBotInfoRequest) (tg.BotInfoClass, error)
	BotsGetBotCommandsFn                    func(ctx context.Context, req *tg.BotsGetBotCommandsRequest) (tg.TLObject, error)
	BotsGetBotMenuButtonFn                  func(ctx context.Context, req *tg.BotsGetBotMenuButtonRequest) (tg.BotMenuButtonClass, error)
	BotsRequestWebViewButtonFn              func(ctx context.Context, req *tg.BotsRequestWebViewButtonRequest) (*tg.BotsRequestedButton, error)
	BotsResetBotCommandsFn                  func(ctx context.Context, req *tg.BotsResetBotCommandsRequest) (bool, error)
	BotsSendCustomRequestFn                 func(ctx context.Context, req *tg.BotsSendCustomRequestRequest) (*tg.DataJSON, error)
	BotsSetBotBroadcastDefaultAdminRightsFn func(ctx context.Context, req *tg.BotsSetBotBroadcastDefaultAdminRightsRequest) (bool, error)
	BotsSetBotCommandsFn                    func(ctx context.Context, req *tg.BotsSetBotCommandsRequest) (bool, error)
	BotsSetBotGroupDefaultAdminRightsFn     func(ctx context.Context, req *tg.BotsSetBotGroupDefaultAdminRightsRequest) (bool, error)
	BotsSetBotInfoFn                        func(ctx context.Context, req *tg.BotsSetBotInfoRequest) (bool, error)
	BotsSetBotMenuButtonFn                  func(ctx context.Context, req *tg.BotsSetBotMenuButtonRequest) (bool, error)
	BotsSetCustomVerificationFn             func(ctx context.Context, req *tg.BotsSetCustomVerificationRequest) (bool, error)
	BotsSetJoinChatResultsFn                func(ctx context.Context, req *tg.BotsSetJoinChatResultsRequest) (bool, error)
	BotsUpdateUserEmojiStatusFn             func(ctx context.Context, req *tg.BotsUpdateUserEmojiStatusRequest) (bool, error)
	ChannelsDeleteMessagesFn                func(ctx context.Context, req *tg.ChannelsDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error)
	ChannelsDeleteParticipantHistoryFn      func(ctx context.Context, req *tg.ChannelsDeleteParticipantHistoryRequest) (*tg.MessagesAffectedHistory, error)
	ChannelsEditAdminFn                     func(ctx context.Context, req *tg.ChannelsEditAdminRequest) (tg.UpdatesClass, error)
	ChannelsEditBannedFn                    func(ctx context.Context, req *tg.ChannelsEditBannedRequest) (tg.UpdatesClass, error)
	ChannelsEditPhotoFn                     func(ctx context.Context, req *tg.ChannelsEditPhotoRequest) (tg.UpdatesClass, error)
	ChannelsEditTitleFn                     func(ctx context.Context, req *tg.ChannelsEditTitleRequest) (tg.UpdatesClass, error)
	ChannelsGetFullChannelFn                func(ctx context.Context, req *tg.ChannelsGetFullChannelRequest) (tg.ChatFullClass, error)
	ChannelsGetMessagesFn                   func(ctx context.Context, req *tg.ChannelsGetMessagesRequest) (tg.MessagesClass, error)
	ChannelsGetParticipantFn                func(ctx context.Context, req *tg.ChannelsGetParticipantRequest) (tg.ChannelParticipantClass, error)
	ChannelsGetParticipantsFn               func(ctx context.Context, req *tg.ChannelsGetParticipantsRequest) (tg.ChannelParticipantsClass, error)
	ChannelsLeaveChannelFn                  func(ctx context.Context, req *tg.ChannelsLeaveChannelRequest) (tg.UpdatesClass, error)
	ChannelsSetStickersFn                   func(ctx context.Context, req *tg.ChannelsSetStickersRequest) (bool, error)
	ContactsResolveUsernameFn               func(ctx context.Context, req *tg.ContactsResolveUsernameRequest) (*tg.ContactsResolvedPeer, error)
	HelpSetBotUpdatesStatusFn               func(ctx context.Context, req *tg.HelpSetBotUpdatesStatusRequest) (bool, error)
	InvokeWithBusinessConnectionFn          func(ctx context.Context, req *tg.InvokeWithBusinessConnectionRequest) (tg.TLObject, error)
	MessagesCreateForumTopicFn              func(ctx context.Context, req *tg.MessagesCreateForumTopicRequest) (tg.UpdatesClass, error)
	MessagesDeleteChatUserFn                func(ctx context.Context, req *tg.MessagesDeleteChatUserRequest) (tg.UpdatesClass, error)
	MessagesDeleteMessagesFn                func(ctx context.Context, req *tg.MessagesDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error)
	MessagesDeleteParticipantReactionFn     func(ctx context.Context, req *tg.MessagesDeleteParticipantReactionRequest) (tg.UpdatesClass, error)
	MessagesDeleteParticipantReactionsFn    func(ctx context.Context, req *tg.MessagesDeleteParticipantReactionsRequest) (bool, error)
	MessagesDeleteTopicHistoryFn            func(ctx context.Context, req *tg.MessagesDeleteTopicHistoryRequest) (*tg.MessagesAffectedHistory, error)
	MessagesEditChatAboutFn                 func(ctx context.Context, req *tg.MessagesEditChatAboutRequest) (bool, error)
	MessagesEditChatDefaultBannedRightsFn   func(ctx context.Context, req *tg.MessagesEditChatDefaultBannedRightsRequest) (tg.UpdatesClass, error)
	MessagesEditChatParticipantRankFn       func(ctx context.Context, req *tg.MessagesEditChatParticipantRankRequest) (tg.UpdatesClass, error)
	MessagesEditChatPhotoFn                 func(ctx context.Context, req *tg.MessagesEditChatPhotoRequest) (tg.UpdatesClass, error)
	MessagesEditChatTitleFn                 func(ctx context.Context, req *tg.MessagesEditChatTitleRequest) (tg.UpdatesClass, error)
	MessagesEditExportedChatInviteFn        func(ctx context.Context, req *tg.MessagesEditExportedChatInviteRequest) (tg.ExportedChatInviteClass, error)
	MessagesEditForumTopicFn                func(ctx context.Context, req *tg.MessagesEditForumTopicRequest) (tg.UpdatesClass, error)
	MessagesEditInlineBotMessageFn          func(ctx context.Context, req *tg.MessagesEditInlineBotMessageRequest) (bool, error)
	MessagesEditMessageFn                   func(ctx context.Context, req *tg.MessagesEditMessageRequest) (tg.UpdatesClass, error)
	MessagesExportChatInviteFn              func(ctx context.Context, req *tg.MessagesExportChatInviteRequest) (tg.ExportedChatInviteClass, error)
	MessagesForwardMessagesFn               func(ctx context.Context, req *tg.MessagesForwardMessagesRequest) (tg.UpdatesClass, error)
	MessagesGetCustomEmojiDocumentsFn       func(ctx context.Context, req *tg.MessagesGetCustomEmojiDocumentsRequest) (tg.TLObject, error)
	MessagesGetDialogsFn                    func(ctx context.Context, req *tg.MessagesGetDialogsRequest) (tg.DialogsClass, error)
	MessagesGetFullChatFn                   func(ctx context.Context, req *tg.MessagesGetFullChatRequest) (tg.ChatFullClass, error)
	MessagesGetGameHighScoresFn             func(ctx context.Context, req *tg.MessagesGetGameHighScoresRequest) (*tg.MessagesHighScores, error)
	MessagesGetInlineGameHighScoresFn       func(ctx context.Context, req *tg.MessagesGetInlineGameHighScoresRequest) (*tg.MessagesHighScores, error)
	MessagesGetMessagesFn                   func(ctx context.Context, req *tg.MessagesGetMessagesRequest) (tg.MessagesClass, error)
	MessagesGetPersonalChannelHistoryFn     func(ctx context.Context, req *tg.MessagesGetPersonalChannelHistoryRequest) (tg.MessagesClass, error)
	MessagesGetStickerSetFn                 func(ctx context.Context, req *tg.MessagesGetStickerSetRequest) (tg.StickerSetClass, error)
	MessagesHideChatJoinRequestFn           func(ctx context.Context, req *tg.MessagesHideChatJoinRequestRequest) (tg.UpdatesClass, error)
	MessagesSavePreparedInlineMessageFn     func(ctx context.Context, req *tg.MessagesSavePreparedInlineMessageRequest) (*tg.MessagesBotPreparedInlineMessage, error)
	MessagesSendMediaFn                     func(ctx context.Context, req *tg.MessagesSendMediaRequest) (tg.UpdatesClass, error)
	MessagesSendMessageFn                   func(ctx context.Context, req *tg.MessagesSendMessageRequest) (tg.UpdatesClass, error)
	MessagesSendMultiMediaFn                func(ctx context.Context, req *tg.MessagesSendMultiMediaRequest) (tg.UpdatesClass, error)
	MessagesSendReactionFn                  func(ctx context.Context, req *tg.MessagesSendReactionRequest) (tg.UpdatesClass, error)
	MessagesSendWebViewResultMessageFn      func(ctx context.Context, req *tg.MessagesSendWebViewResultMessageRequest) (*tg.WebViewMessageSent, error)
	MessagesSetBotCallbackAnswerFn          func(ctx context.Context, req *tg.MessagesSetBotCallbackAnswerRequest) (bool, error)
	MessagesSetBotGuestChatResultFn         func(ctx context.Context, req *tg.MessagesSetBotGuestChatResultRequest) (tg.InputBotInlineMessageIDClass, error)
	MessagesSetGameScoreFn                  func(ctx context.Context, req *tg.MessagesSetGameScoreRequest) (tg.UpdatesClass, error)
	MessagesSetInlineBotResultsFn           func(ctx context.Context, req *tg.MessagesSetInlineBotResultsRequest) (bool, error)
	MessagesSetInlineGameScoreFn            func(ctx context.Context, req *tg.MessagesSetInlineGameScoreRequest) (bool, error)
	MessagesSetTypingFn                     func(ctx context.Context, req *tg.MessagesSetTypingRequest) (bool, error)
	MessagesToggleSuggestedPostApprovalFn   func(ctx context.Context, req *tg.MessagesToggleSuggestedPostApprovalRequest) (tg.UpdatesClass, error)
	MessagesUnpinAllMessagesFn              func(ctx context.Context, req *tg.MessagesUnpinAllMessagesRequest) (*tg.MessagesAffectedHistory, error)
	MessagesUpdatePinnedMessageFn           func(ctx context.Context, req *tg.MessagesUpdatePinnedMessageRequest) (tg.UpdatesClass, error)
	MessagesUploadMediaFn                   func(ctx context.Context, req *tg.MessagesUploadMediaRequest) (tg.MessageMediaClass, error)
	PaymentsChangeStarsSubscriptionFn       func(ctx context.Context, req *tg.PaymentsChangeStarsSubscriptionRequest) (bool, error)
	PaymentsConvertStarGiftFn               func(ctx context.Context, req *tg.PaymentsConvertStarGiftRequest) (bool, error)
	PaymentsExportInvoiceFn                 func(ctx context.Context, req *tg.PaymentsExportInvoiceRequest) (*tg.PaymentsExportedInvoice, error)
	PaymentsGetPaymentFormFn                func(ctx context.Context, req *tg.PaymentsGetPaymentFormRequest) (tg.PaymentFormClass, error)
	PaymentsGetSavedStarGiftsFn             func(ctx context.Context, req *tg.PaymentsGetSavedStarGiftsRequest) (*tg.PaymentsSavedStarGifts, error)
	PaymentsGetStarGiftsFn                  func(ctx context.Context, req *tg.PaymentsGetStarGiftsRequest) (tg.StarGiftsClass, error)
	PaymentsGetStarsTransactionsFn          func(ctx context.Context, req *tg.PaymentsGetStarsTransactionsRequest) (*tg.PaymentsStarsStatus, error)
	PaymentsRefundStarsChargeFn             func(ctx context.Context, req *tg.PaymentsRefundStarsChargeRequest) (tg.UpdatesClass, error)
	PaymentsSendStarsFormFn                 func(ctx context.Context, req *tg.PaymentsSendStarsFormRequest) (tg.PaymentResultClass, error)
	PaymentsTransferStarGiftFn              func(ctx context.Context, req *tg.PaymentsTransferStarGiftRequest) (tg.UpdatesClass, error)
	PaymentsUpgradeStarGiftFn               func(ctx context.Context, req *tg.PaymentsUpgradeStarGiftRequest) (tg.UpdatesClass, error)
	PhotosDeletePhotosFn                    func(ctx context.Context, req *tg.PhotosDeletePhotosRequest) (tg.TLObject, error)
	PhotosGetUserPhotosFn                   func(ctx context.Context, req *tg.PhotosGetUserPhotosRequest) (tg.PhotosClass, error)
	PhotosUploadProfilePhotoFn              func(ctx context.Context, req *tg.PhotosUploadProfilePhotoRequest) (tg.PhotoClass, error)
	PremiumGetUserBoostsFn                  func(ctx context.Context, req *tg.PremiumGetUserBoostsRequest) (*tg.PremiumBoostsList, error)
	StickersAddStickerToSetFn               func(ctx context.Context, req *tg.StickersAddStickerToSetRequest) (tg.StickerSetClass, error)
	StickersChangeStickerFn                 func(ctx context.Context, req *tg.StickersChangeStickerRequest) (tg.StickerSetClass, error)
	StickersChangeStickerPositionFn         func(ctx context.Context, req *tg.StickersChangeStickerPositionRequest) (tg.StickerSetClass, error)
	StickersCreateStickerSetFn              func(ctx context.Context, req *tg.StickersCreateStickerSetRequest) (tg.StickerSetClass, error)
	StickersDeleteStickerSetFn              func(ctx context.Context, req *tg.StickersDeleteStickerSetRequest) (bool, error)
	StickersRemoveStickerFromSetFn          func(ctx context.Context, req *tg.StickersRemoveStickerFromSetRequest) (tg.StickerSetClass, error)
	StickersRenameStickerSetFn              func(ctx context.Context, req *tg.StickersRenameStickerSetRequest) (tg.StickerSetClass, error)
	StickersReplaceStickerFn                func(ctx context.Context, req *tg.StickersReplaceStickerRequest) (tg.StickerSetClass, error)
	StickersSetStickerSetThumbFn            func(ctx context.Context, req *tg.StickersSetStickerSetThumbRequest) (tg.StickerSetClass, error)
	StoriesDeleteStoriesFn                  func(ctx context.Context, req *tg.StoriesDeleteStoriesRequest) (tg.TLObject, error)
	StoriesEditStoryFn                      func(ctx context.Context, req *tg.StoriesEditStoryRequest) (tg.UpdatesClass, error)
	StoriesSendStoryFn                      func(ctx context.Context, req *tg.StoriesSendStoryRequest) (tg.UpdatesClass, error)
	UploadGetFileFn                         func(ctx context.Context, req *tg.UploadGetFileRequest) (tg.FileClass, error)
	UploadGetWebFileFn                      func(ctx context.Context, req *tg.UploadGetWebFileRequest) (*tg.UploadWebFile, error)
	UploadSaveBigFilePartFn                 func(ctx context.Context, req *tg.UploadSaveBigFilePartRequest) (bool, error)
	UploadSaveFilePartFn                    func(ctx context.Context, req *tg.UploadSaveFilePartRequest) (bool, error)
	UsersGetFullUserFn                      func(ctx context.Context, req *tg.UsersGetFullUserRequest) (tg.UserFullClass, error)
	UsersGetSavedMusicFn                    func(ctx context.Context, req *tg.UsersGetSavedMusicRequest) (tg.SavedMusicClass, error)
	UsersSetSecureValueErrorsFn             func(ctx context.Context, req *tg.UsersSetSecureValueErrorsRequest) (bool, error)
}

func (f *fakeRPC) AccountGetBotBusinessConnection(ctx context.Context, req *tg.AccountGetBotBusinessConnectionRequest) (tg.UpdatesClass, error) {
	if f.AccountGetBotBusinessConnectionFn != nil {
		return f.AccountGetBotBusinessConnectionFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) AuthLogOut(ctx context.Context) (*tg.AuthLoggedOut, error) {
	if f.AuthLogOutFn != nil {
		return f.AuthLogOutFn(ctx)
	}
	return nil, nil
}

func (f *fakeRPC) BotsAnswerWebhookJSONQuery(ctx context.Context, req *tg.BotsAnswerWebhookJSONQueryRequest) (bool, error) {
	if f.BotsAnswerWebhookJSONQueryFn != nil {
		return f.BotsAnswerWebhookJSONQueryFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsEditAccessSettings(ctx context.Context, req *tg.BotsEditAccessSettingsRequest) (bool, error) {
	if f.BotsEditAccessSettingsFn != nil {
		return f.BotsEditAccessSettingsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsExportBotToken(ctx context.Context, req *tg.BotsExportBotTokenRequest) (*tg.BotsExportedBotToken, error) {
	if f.BotsExportBotTokenFn != nil {
		return f.BotsExportBotTokenFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) BotsGetAccessSettings(ctx context.Context, req *tg.BotsGetAccessSettingsRequest) (*tg.BotsAccessSettings, error) {
	if f.BotsGetAccessSettingsFn != nil {
		return f.BotsGetAccessSettingsFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) BotsGetBotInfo(ctx context.Context, req *tg.BotsGetBotInfoRequest) (tg.BotInfoClass, error) {
	if f.BotsGetBotInfoFn != nil {
		return f.BotsGetBotInfoFn(ctx, req)
	}
	return (tg.BotInfoClass)(nil), nil
}

func (f *fakeRPC) BotsGetBotCommands(ctx context.Context, req *tg.BotsGetBotCommandsRequest) (tg.TLObject, error) {
	if f.BotsGetBotCommandsFn != nil {
		return f.BotsGetBotCommandsFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) BotsGetBotMenuButton(ctx context.Context, req *tg.BotsGetBotMenuButtonRequest) (tg.BotMenuButtonClass, error) {
	if f.BotsGetBotMenuButtonFn != nil {
		return f.BotsGetBotMenuButtonFn(ctx, req)
	}
	return (tg.BotMenuButtonClass)(nil), nil
}

func (f *fakeRPC) BotsRequestWebViewButton(ctx context.Context, req *tg.BotsRequestWebViewButtonRequest) (*tg.BotsRequestedButton, error) {
	if f.BotsRequestWebViewButtonFn != nil {
		return f.BotsRequestWebViewButtonFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) BotsResetBotCommands(ctx context.Context, req *tg.BotsResetBotCommandsRequest) (bool, error) {
	if f.BotsResetBotCommandsFn != nil {
		return f.BotsResetBotCommandsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSendCustomRequest(ctx context.Context, req *tg.BotsSendCustomRequestRequest) (*tg.DataJSON, error) {
	if f.BotsSendCustomRequestFn != nil {
		return f.BotsSendCustomRequestFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) BotsSetBotBroadcastDefaultAdminRights(ctx context.Context, req *tg.BotsSetBotBroadcastDefaultAdminRightsRequest) (bool, error) {
	if f.BotsSetBotBroadcastDefaultAdminRightsFn != nil {
		return f.BotsSetBotBroadcastDefaultAdminRightsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSetBotCommands(ctx context.Context, req *tg.BotsSetBotCommandsRequest) (bool, error) {
	if f.BotsSetBotCommandsFn != nil {
		return f.BotsSetBotCommandsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSetBotGroupDefaultAdminRights(ctx context.Context, req *tg.BotsSetBotGroupDefaultAdminRightsRequest) (bool, error) {
	if f.BotsSetBotGroupDefaultAdminRightsFn != nil {
		return f.BotsSetBotGroupDefaultAdminRightsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSetBotInfo(ctx context.Context, req *tg.BotsSetBotInfoRequest) (bool, error) {
	if f.BotsSetBotInfoFn != nil {
		return f.BotsSetBotInfoFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSetBotMenuButton(ctx context.Context, req *tg.BotsSetBotMenuButtonRequest) (bool, error) {
	if f.BotsSetBotMenuButtonFn != nil {
		return f.BotsSetBotMenuButtonFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSetCustomVerification(ctx context.Context, req *tg.BotsSetCustomVerificationRequest) (bool, error) {
	if f.BotsSetCustomVerificationFn != nil {
		return f.BotsSetCustomVerificationFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsSetJoinChatResults(ctx context.Context, req *tg.BotsSetJoinChatResultsRequest) (bool, error) {
	if f.BotsSetJoinChatResultsFn != nil {
		return f.BotsSetJoinChatResultsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) BotsUpdateUserEmojiStatus(ctx context.Context, req *tg.BotsUpdateUserEmojiStatusRequest) (bool, error) {
	if f.BotsUpdateUserEmojiStatusFn != nil {
		return f.BotsUpdateUserEmojiStatusFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) ChannelsDeleteMessages(ctx context.Context, req *tg.ChannelsDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error) {
	if f.ChannelsDeleteMessagesFn != nil {
		return f.ChannelsDeleteMessagesFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) ChannelsDeleteParticipantHistory(ctx context.Context, req *tg.ChannelsDeleteParticipantHistoryRequest) (*tg.MessagesAffectedHistory, error) {
	if f.ChannelsDeleteParticipantHistoryFn != nil {
		return f.ChannelsDeleteParticipantHistoryFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) ChannelsEditAdmin(ctx context.Context, req *tg.ChannelsEditAdminRequest) (tg.UpdatesClass, error) {
	if f.ChannelsEditAdminFn != nil {
		return f.ChannelsEditAdminFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) ChannelsEditBanned(ctx context.Context, req *tg.ChannelsEditBannedRequest) (tg.UpdatesClass, error) {
	if f.ChannelsEditBannedFn != nil {
		return f.ChannelsEditBannedFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) ChannelsEditPhoto(ctx context.Context, req *tg.ChannelsEditPhotoRequest) (tg.UpdatesClass, error) {
	if f.ChannelsEditPhotoFn != nil {
		return f.ChannelsEditPhotoFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) ChannelsEditTitle(ctx context.Context, req *tg.ChannelsEditTitleRequest) (tg.UpdatesClass, error) {
	if f.ChannelsEditTitleFn != nil {
		return f.ChannelsEditTitleFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) ChannelsGetFullChannel(ctx context.Context, req *tg.ChannelsGetFullChannelRequest) (tg.ChatFullClass, error) {
	if f.ChannelsGetFullChannelFn != nil {
		return f.ChannelsGetFullChannelFn(ctx, req)
	}
	return (tg.ChatFullClass)(nil), nil
}

func (f *fakeRPC) ChannelsGetMessages(ctx context.Context, req *tg.ChannelsGetMessagesRequest) (tg.MessagesClass, error) {
	if f.ChannelsGetMessagesFn != nil {
		return f.ChannelsGetMessagesFn(ctx, req)
	}
	return (tg.MessagesClass)(nil), nil
}

func (f *fakeRPC) ChannelsGetParticipant(ctx context.Context, req *tg.ChannelsGetParticipantRequest) (tg.ChannelParticipantClass, error) {
	if f.ChannelsGetParticipantFn != nil {
		return f.ChannelsGetParticipantFn(ctx, req)
	}
	return (tg.ChannelParticipantClass)(nil), nil
}

func (f *fakeRPC) ChannelsGetParticipants(ctx context.Context, req *tg.ChannelsGetParticipantsRequest) (tg.ChannelParticipantsClass, error) {
	if f.ChannelsGetParticipantsFn != nil {
		return f.ChannelsGetParticipantsFn(ctx, req)
	}
	return (tg.ChannelParticipantsClass)(nil), nil
}

func (f *fakeRPC) ChannelsLeaveChannel(ctx context.Context, req *tg.ChannelsLeaveChannelRequest) (tg.UpdatesClass, error) {
	if f.ChannelsLeaveChannelFn != nil {
		return f.ChannelsLeaveChannelFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) ChannelsSetStickers(ctx context.Context, req *tg.ChannelsSetStickersRequest) (bool, error) {
	if f.ChannelsSetStickersFn != nil {
		return f.ChannelsSetStickersFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) ContactsResolveUsername(ctx context.Context, req *tg.ContactsResolveUsernameRequest) (*tg.ContactsResolvedPeer, error) {
	if f.ContactsResolveUsernameFn != nil {
		return f.ContactsResolveUsernameFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) HelpSetBotUpdatesStatus(ctx context.Context, req *tg.HelpSetBotUpdatesStatusRequest) (bool, error) {
	if f.HelpSetBotUpdatesStatusFn != nil {
		return f.HelpSetBotUpdatesStatusFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) InvokeWithBusinessConnection(ctx context.Context, req *tg.InvokeWithBusinessConnectionRequest) (tg.TLObject, error) {
	if f.InvokeWithBusinessConnectionFn != nil {
		return f.InvokeWithBusinessConnectionFn(ctx, req)
	}
	return (tg.TLObject)(nil), nil
}

func (f *fakeRPC) MessagesCreateForumTopic(ctx context.Context, req *tg.MessagesCreateForumTopicRequest) (tg.UpdatesClass, error) {
	if f.MessagesCreateForumTopicFn != nil {
		return f.MessagesCreateForumTopicFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesDeleteChatUser(ctx context.Context, req *tg.MessagesDeleteChatUserRequest) (tg.UpdatesClass, error) {
	if f.MessagesDeleteChatUserFn != nil {
		return f.MessagesDeleteChatUserFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesDeleteMessages(ctx context.Context, req *tg.MessagesDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error) {
	if f.MessagesDeleteMessagesFn != nil {
		return f.MessagesDeleteMessagesFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesDeleteParticipantReaction(ctx context.Context, req *tg.MessagesDeleteParticipantReactionRequest) (tg.UpdatesClass, error) {
	if f.MessagesDeleteParticipantReactionFn != nil {
		return f.MessagesDeleteParticipantReactionFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesDeleteParticipantReactions(ctx context.Context, req *tg.MessagesDeleteParticipantReactionsRequest) (bool, error) {
	if f.MessagesDeleteParticipantReactionsFn != nil {
		return f.MessagesDeleteParticipantReactionsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesDeleteTopicHistory(ctx context.Context, req *tg.MessagesDeleteTopicHistoryRequest) (*tg.MessagesAffectedHistory, error) {
	if f.MessagesDeleteTopicHistoryFn != nil {
		return f.MessagesDeleteTopicHistoryFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesEditChatAbout(ctx context.Context, req *tg.MessagesEditChatAboutRequest) (bool, error) {
	if f.MessagesEditChatAboutFn != nil {
		return f.MessagesEditChatAboutFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesEditChatDefaultBannedRights(ctx context.Context, req *tg.MessagesEditChatDefaultBannedRightsRequest) (tg.UpdatesClass, error) {
	if f.MessagesEditChatDefaultBannedRightsFn != nil {
		return f.MessagesEditChatDefaultBannedRightsFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesEditChatParticipantRank(ctx context.Context, req *tg.MessagesEditChatParticipantRankRequest) (tg.UpdatesClass, error) {
	if f.MessagesEditChatParticipantRankFn != nil {
		return f.MessagesEditChatParticipantRankFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesEditChatPhoto(ctx context.Context, req *tg.MessagesEditChatPhotoRequest) (tg.UpdatesClass, error) {
	if f.MessagesEditChatPhotoFn != nil {
		return f.MessagesEditChatPhotoFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesEditChatTitle(ctx context.Context, req *tg.MessagesEditChatTitleRequest) (tg.UpdatesClass, error) {
	if f.MessagesEditChatTitleFn != nil {
		return f.MessagesEditChatTitleFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesEditExportedChatInvite(ctx context.Context, req *tg.MessagesEditExportedChatInviteRequest) (tg.ExportedChatInviteClass, error) {
	if f.MessagesEditExportedChatInviteFn != nil {
		return f.MessagesEditExportedChatInviteFn(ctx, req)
	}
	return (tg.ExportedChatInviteClass)(nil), nil
}

func (f *fakeRPC) MessagesEditForumTopic(ctx context.Context, req *tg.MessagesEditForumTopicRequest) (tg.UpdatesClass, error) {
	if f.MessagesEditForumTopicFn != nil {
		return f.MessagesEditForumTopicFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesEditInlineBotMessage(ctx context.Context, req *tg.MessagesEditInlineBotMessageRequest) (bool, error) {
	if f.MessagesEditInlineBotMessageFn != nil {
		return f.MessagesEditInlineBotMessageFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesEditMessage(ctx context.Context, req *tg.MessagesEditMessageRequest) (tg.UpdatesClass, error) {
	if f.MessagesEditMessageFn != nil {
		return f.MessagesEditMessageFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesExportChatInvite(ctx context.Context, req *tg.MessagesExportChatInviteRequest) (tg.ExportedChatInviteClass, error) {
	if f.MessagesExportChatInviteFn != nil {
		return f.MessagesExportChatInviteFn(ctx, req)
	}
	return (tg.ExportedChatInviteClass)(nil), nil
}

func (f *fakeRPC) MessagesForwardMessages(ctx context.Context, req *tg.MessagesForwardMessagesRequest) (tg.UpdatesClass, error) {
	if f.MessagesForwardMessagesFn != nil {
		return f.MessagesForwardMessagesFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesGetCustomEmojiDocuments(ctx context.Context, req *tg.MessagesGetCustomEmojiDocumentsRequest) (tg.TLObject, error) {
	if f.MessagesGetCustomEmojiDocumentsFn != nil {
		return f.MessagesGetCustomEmojiDocumentsFn(ctx, req)
	}
	return (tg.TLObject)(nil), nil
}

func (f *fakeRPC) MessagesGetDialogs(ctx context.Context, req *tg.MessagesGetDialogsRequest) (tg.DialogsClass, error) {
	if f.MessagesGetDialogsFn != nil {
		return f.MessagesGetDialogsFn(ctx, req)
	}
	return (tg.DialogsClass)(nil), nil
}

func (f *fakeRPC) MessagesGetFullChat(ctx context.Context, req *tg.MessagesGetFullChatRequest) (tg.ChatFullClass, error) {
	if f.MessagesGetFullChatFn != nil {
		return f.MessagesGetFullChatFn(ctx, req)
	}
	return (tg.ChatFullClass)(nil), nil
}

func (f *fakeRPC) MessagesGetGameHighScores(ctx context.Context, req *tg.MessagesGetGameHighScoresRequest) (*tg.MessagesHighScores, error) {
	if f.MessagesGetGameHighScoresFn != nil {
		return f.MessagesGetGameHighScoresFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesGetInlineGameHighScores(ctx context.Context, req *tg.MessagesGetInlineGameHighScoresRequest) (*tg.MessagesHighScores, error) {
	if f.MessagesGetInlineGameHighScoresFn != nil {
		return f.MessagesGetInlineGameHighScoresFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesGetMessages(ctx context.Context, req *tg.MessagesGetMessagesRequest) (tg.MessagesClass, error) {
	if f.MessagesGetMessagesFn != nil {
		return f.MessagesGetMessagesFn(ctx, req)
	}
	return (tg.MessagesClass)(nil), nil
}

func (f *fakeRPC) MessagesGetPersonalChannelHistory(ctx context.Context, req *tg.MessagesGetPersonalChannelHistoryRequest) (tg.MessagesClass, error) {
	if f.MessagesGetPersonalChannelHistoryFn != nil {
		return f.MessagesGetPersonalChannelHistoryFn(ctx, req)
	}
	return (tg.MessagesClass)(nil), nil
}

func (f *fakeRPC) MessagesGetStickerSet(ctx context.Context, req *tg.MessagesGetStickerSetRequest) (tg.StickerSetClass, error) {
	if f.MessagesGetStickerSetFn != nil {
		return f.MessagesGetStickerSetFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) MessagesHideChatJoinRequest(ctx context.Context, req *tg.MessagesHideChatJoinRequestRequest) (tg.UpdatesClass, error) {
	if f.MessagesHideChatJoinRequestFn != nil {
		return f.MessagesHideChatJoinRequestFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesSavePreparedInlineMessage(ctx context.Context, req *tg.MessagesSavePreparedInlineMessageRequest) (*tg.MessagesBotPreparedInlineMessage, error) {
	if f.MessagesSavePreparedInlineMessageFn != nil {
		return f.MessagesSavePreparedInlineMessageFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesSendMedia(ctx context.Context, req *tg.MessagesSendMediaRequest) (tg.UpdatesClass, error) {
	if f.MessagesSendMediaFn != nil {
		return f.MessagesSendMediaFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesSendMessage(ctx context.Context, req *tg.MessagesSendMessageRequest) (tg.UpdatesClass, error) {
	if f.MessagesSendMessageFn != nil {
		return f.MessagesSendMessageFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesSendMultiMedia(ctx context.Context, req *tg.MessagesSendMultiMediaRequest) (tg.UpdatesClass, error) {
	if f.MessagesSendMultiMediaFn != nil {
		return f.MessagesSendMultiMediaFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesSendReaction(ctx context.Context, req *tg.MessagesSendReactionRequest) (tg.UpdatesClass, error) {
	if f.MessagesSendReactionFn != nil {
		return f.MessagesSendReactionFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesSendWebViewResultMessage(ctx context.Context, req *tg.MessagesSendWebViewResultMessageRequest) (*tg.WebViewMessageSent, error) {
	if f.MessagesSendWebViewResultMessageFn != nil {
		return f.MessagesSendWebViewResultMessageFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesSetBotCallbackAnswer(ctx context.Context, req *tg.MessagesSetBotCallbackAnswerRequest) (bool, error) {
	if f.MessagesSetBotCallbackAnswerFn != nil {
		return f.MessagesSetBotCallbackAnswerFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesSetBotGuestChatResult(ctx context.Context, req *tg.MessagesSetBotGuestChatResultRequest) (tg.InputBotInlineMessageIDClass, error) {
	if f.MessagesSetBotGuestChatResultFn != nil {
		return f.MessagesSetBotGuestChatResultFn(ctx, req)
	}
	return (tg.InputBotInlineMessageIDClass)(nil), nil
}

func (f *fakeRPC) MessagesSetGameScore(ctx context.Context, req *tg.MessagesSetGameScoreRequest) (tg.UpdatesClass, error) {
	if f.MessagesSetGameScoreFn != nil {
		return f.MessagesSetGameScoreFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesSetInlineBotResults(ctx context.Context, req *tg.MessagesSetInlineBotResultsRequest) (bool, error) {
	if f.MessagesSetInlineBotResultsFn != nil {
		return f.MessagesSetInlineBotResultsFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesSetInlineGameScore(ctx context.Context, req *tg.MessagesSetInlineGameScoreRequest) (bool, error) {
	if f.MessagesSetInlineGameScoreFn != nil {
		return f.MessagesSetInlineGameScoreFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesSetTyping(ctx context.Context, req *tg.MessagesSetTypingRequest) (bool, error) {
	if f.MessagesSetTypingFn != nil {
		return f.MessagesSetTypingFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) MessagesToggleSuggestedPostApproval(ctx context.Context, req *tg.MessagesToggleSuggestedPostApprovalRequest) (tg.UpdatesClass, error) {
	if f.MessagesToggleSuggestedPostApprovalFn != nil {
		return f.MessagesToggleSuggestedPostApprovalFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesUnpinAllMessages(ctx context.Context, req *tg.MessagesUnpinAllMessagesRequest) (*tg.MessagesAffectedHistory, error) {
	if f.MessagesUnpinAllMessagesFn != nil {
		return f.MessagesUnpinAllMessagesFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) MessagesUpdatePinnedMessage(ctx context.Context, req *tg.MessagesUpdatePinnedMessageRequest) (tg.UpdatesClass, error) {
	if f.MessagesUpdatePinnedMessageFn != nil {
		return f.MessagesUpdatePinnedMessageFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) MessagesUploadMedia(ctx context.Context, req *tg.MessagesUploadMediaRequest) (tg.MessageMediaClass, error) {
	if f.MessagesUploadMediaFn != nil {
		return f.MessagesUploadMediaFn(ctx, req)
	}
	return (tg.MessageMediaClass)(nil), nil
}

func (f *fakeRPC) PaymentsChangeStarsSubscription(ctx context.Context, req *tg.PaymentsChangeStarsSubscriptionRequest) (bool, error) {
	if f.PaymentsChangeStarsSubscriptionFn != nil {
		return f.PaymentsChangeStarsSubscriptionFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) PaymentsConvertStarGift(ctx context.Context, req *tg.PaymentsConvertStarGiftRequest) (bool, error) {
	if f.PaymentsConvertStarGiftFn != nil {
		return f.PaymentsConvertStarGiftFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) PaymentsExportInvoice(ctx context.Context, req *tg.PaymentsExportInvoiceRequest) (*tg.PaymentsExportedInvoice, error) {
	if f.PaymentsExportInvoiceFn != nil {
		return f.PaymentsExportInvoiceFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) PaymentsGetPaymentForm(ctx context.Context, req *tg.PaymentsGetPaymentFormRequest) (tg.PaymentFormClass, error) {
	if f.PaymentsGetPaymentFormFn != nil {
		return f.PaymentsGetPaymentFormFn(ctx, req)
	}
	return (tg.PaymentFormClass)(nil), nil
}

func (f *fakeRPC) PaymentsGetSavedStarGifts(ctx context.Context, req *tg.PaymentsGetSavedStarGiftsRequest) (*tg.PaymentsSavedStarGifts, error) {
	if f.PaymentsGetSavedStarGiftsFn != nil {
		return f.PaymentsGetSavedStarGiftsFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) PaymentsGetStarGifts(ctx context.Context, req *tg.PaymentsGetStarGiftsRequest) (tg.StarGiftsClass, error) {
	if f.PaymentsGetStarGiftsFn != nil {
		return f.PaymentsGetStarGiftsFn(ctx, req)
	}
	return (tg.StarGiftsClass)(nil), nil
}

func (f *fakeRPC) PaymentsGetStarsTransactions(ctx context.Context, req *tg.PaymentsGetStarsTransactionsRequest) (*tg.PaymentsStarsStatus, error) {
	if f.PaymentsGetStarsTransactionsFn != nil {
		return f.PaymentsGetStarsTransactionsFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) PaymentsRefundStarsCharge(ctx context.Context, req *tg.PaymentsRefundStarsChargeRequest) (tg.UpdatesClass, error) {
	if f.PaymentsRefundStarsChargeFn != nil {
		return f.PaymentsRefundStarsChargeFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) PaymentsSendStarsForm(ctx context.Context, req *tg.PaymentsSendStarsFormRequest) (tg.PaymentResultClass, error) {
	if f.PaymentsSendStarsFormFn != nil {
		return f.PaymentsSendStarsFormFn(ctx, req)
	}
	return (tg.PaymentResultClass)(nil), nil
}

func (f *fakeRPC) PaymentsTransferStarGift(ctx context.Context, req *tg.PaymentsTransferStarGiftRequest) (tg.UpdatesClass, error) {
	if f.PaymentsTransferStarGiftFn != nil {
		return f.PaymentsTransferStarGiftFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) PaymentsUpgradeStarGift(ctx context.Context, req *tg.PaymentsUpgradeStarGiftRequest) (tg.UpdatesClass, error) {
	if f.PaymentsUpgradeStarGiftFn != nil {
		return f.PaymentsUpgradeStarGiftFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) PhotosDeletePhotos(ctx context.Context, req *tg.PhotosDeletePhotosRequest) (tg.TLObject, error) {
	if f.PhotosDeletePhotosFn != nil {
		return f.PhotosDeletePhotosFn(ctx, req)
	}
	return (tg.TLObject)(nil), nil
}

func (f *fakeRPC) PhotosGetUserPhotos(ctx context.Context, req *tg.PhotosGetUserPhotosRequest) (tg.PhotosClass, error) {
	if f.PhotosGetUserPhotosFn != nil {
		return f.PhotosGetUserPhotosFn(ctx, req)
	}
	return (tg.PhotosClass)(nil), nil
}

func (f *fakeRPC) PhotosUploadProfilePhoto(ctx context.Context, req *tg.PhotosUploadProfilePhotoRequest) (tg.PhotoClass, error) {
	if f.PhotosUploadProfilePhotoFn != nil {
		return f.PhotosUploadProfilePhotoFn(ctx, req)
	}
	return (tg.PhotoClass)(nil), nil
}

func (f *fakeRPC) PremiumGetUserBoosts(ctx context.Context, req *tg.PremiumGetUserBoostsRequest) (*tg.PremiumBoostsList, error) {
	if f.PremiumGetUserBoostsFn != nil {
		return f.PremiumGetUserBoostsFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) StickersAddStickerToSet(ctx context.Context, req *tg.StickersAddStickerToSetRequest) (tg.StickerSetClass, error) {
	if f.StickersAddStickerToSetFn != nil {
		return f.StickersAddStickerToSetFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersChangeSticker(ctx context.Context, req *tg.StickersChangeStickerRequest) (tg.StickerSetClass, error) {
	if f.StickersChangeStickerFn != nil {
		return f.StickersChangeStickerFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersChangeStickerPosition(ctx context.Context, req *tg.StickersChangeStickerPositionRequest) (tg.StickerSetClass, error) {
	if f.StickersChangeStickerPositionFn != nil {
		return f.StickersChangeStickerPositionFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersCreateStickerSet(ctx context.Context, req *tg.StickersCreateStickerSetRequest) (tg.StickerSetClass, error) {
	if f.StickersCreateStickerSetFn != nil {
		return f.StickersCreateStickerSetFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersDeleteStickerSet(ctx context.Context, req *tg.StickersDeleteStickerSetRequest) (bool, error) {
	if f.StickersDeleteStickerSetFn != nil {
		return f.StickersDeleteStickerSetFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) StickersRemoveStickerFromSet(ctx context.Context, req *tg.StickersRemoveStickerFromSetRequest) (tg.StickerSetClass, error) {
	if f.StickersRemoveStickerFromSetFn != nil {
		return f.StickersRemoveStickerFromSetFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersRenameStickerSet(ctx context.Context, req *tg.StickersRenameStickerSetRequest) (tg.StickerSetClass, error) {
	if f.StickersRenameStickerSetFn != nil {
		return f.StickersRenameStickerSetFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersReplaceSticker(ctx context.Context, req *tg.StickersReplaceStickerRequest) (tg.StickerSetClass, error) {
	if f.StickersReplaceStickerFn != nil {
		return f.StickersReplaceStickerFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StickersSetStickerSetThumb(ctx context.Context, req *tg.StickersSetStickerSetThumbRequest) (tg.StickerSetClass, error) {
	if f.StickersSetStickerSetThumbFn != nil {
		return f.StickersSetStickerSetThumbFn(ctx, req)
	}
	return (tg.StickerSetClass)(nil), nil
}

func (f *fakeRPC) StoriesDeleteStories(ctx context.Context, req *tg.StoriesDeleteStoriesRequest) (tg.TLObject, error) {
	if f.StoriesDeleteStoriesFn != nil {
		return f.StoriesDeleteStoriesFn(ctx, req)
	}
	return (tg.TLObject)(nil), nil
}

func (f *fakeRPC) StoriesEditStory(ctx context.Context, req *tg.StoriesEditStoryRequest) (tg.UpdatesClass, error) {
	if f.StoriesEditStoryFn != nil {
		return f.StoriesEditStoryFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) StoriesSendStory(ctx context.Context, req *tg.StoriesSendStoryRequest) (tg.UpdatesClass, error) {
	if f.StoriesSendStoryFn != nil {
		return f.StoriesSendStoryFn(ctx, req)
	}
	return (tg.UpdatesClass)(nil), nil
}

func (f *fakeRPC) UploadGetFile(ctx context.Context, req *tg.UploadGetFileRequest) (tg.FileClass, error) {
	if f.UploadGetFileFn != nil {
		return f.UploadGetFileFn(ctx, req)
	}
	return (tg.FileClass)(nil), nil
}

func (f *fakeRPC) UploadGetWebFile(ctx context.Context, req *tg.UploadGetWebFileRequest) (*tg.UploadWebFile, error) {
	if f.UploadGetWebFileFn != nil {
		return f.UploadGetWebFileFn(ctx, req)
	}
	return nil, nil
}

func (f *fakeRPC) UploadSaveBigFilePart(ctx context.Context, req *tg.UploadSaveBigFilePartRequest) (bool, error) {
	if f.UploadSaveBigFilePartFn != nil {
		return f.UploadSaveBigFilePartFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) UploadSaveFilePart(ctx context.Context, req *tg.UploadSaveFilePartRequest) (bool, error) {
	if f.UploadSaveFilePartFn != nil {
		return f.UploadSaveFilePartFn(ctx, req)
	}
	return false, nil
}

func (f *fakeRPC) UsersGetFullUser(ctx context.Context, req *tg.UsersGetFullUserRequest) (tg.UserFullClass, error) {
	if f.UsersGetFullUserFn != nil {
		return f.UsersGetFullUserFn(ctx, req)
	}
	return (tg.UserFullClass)(nil), nil
}

func (f *fakeRPC) UsersGetSavedMusic(ctx context.Context, req *tg.UsersGetSavedMusicRequest) (tg.SavedMusicClass, error) {
	if f.UsersGetSavedMusicFn != nil {
		return f.UsersGetSavedMusicFn(ctx, req)
	}
	return (tg.SavedMusicClass)(nil), nil
}

func (f *fakeRPC) UsersSetSecureValueErrors(ctx context.Context, req *tg.UsersSetSecureValueErrorsRequest) (bool, error) {
	if f.UsersSetSecureValueErrorsFn != nil {
		return f.UsersSetSecureValueErrorsFn(ctx, req)
	}
	return false, nil
}
