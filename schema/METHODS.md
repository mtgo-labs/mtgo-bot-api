# Bot API Method Index

Generated from schema version **10.1**.

`+` = implemented, `~` = deprecated/extension, `-` = missing.

## Available Methods (131)

| Method | Returns | Status |
|--------|---------|--------|
| `answerCallbackQuery` | `Boolean` | + |
| `answerChatJoinRequestQuery` | `Boolean` | + |
| `answerGuestQuery` | `SentGuestMessage` | + |
| `answerWebAppQuery` | `SentWebAppMessage` | + |
| `approveChatJoinRequest` | `Boolean` | + |
| `banChatMember` | `In` | + |
| `banChatSenderChat` | `Boolean` | + |
| `close` | `The` | + |
| `closeForumTopic` | `Boolean` | + |
| `closeGeneralForumTopic` | `Boolean` | + |
| `convertGiftToStars` | `Boolean` | + |
| `copyMessage` | `MessageId` | + |
| `copyMessages` | `MessageId[]` | + |
| `createChatInviteLink` | `ChatInviteLink` | + |
| `createChatSubscriptionInviteLink` | `ChatInviteLink` | + |
| `createForumTopic` | `ForumTopic` | + |
| `declineChatJoinRequest` | `Boolean` | + |
| `deleteBusinessMessages` | `Boolean` | + |
| `deleteChatPhoto` | `Boolean` | + |
| `deleteChatStickerSet` | `Use` | + |
| `deleteForumTopic` | `Boolean` | + |
| `deleteMyCommands` | `Boolean` | + |
| `deleteStory` | `Boolean` | + |
| `editChatInviteLink` | `ChatInviteLink` | + |
| `editChatSubscriptionInviteLink` | `ChatInviteLink` | + |
| `editForumTopic` | `Boolean` | + |
| `editGeneralForumTopic` | `Boolean` | + |
| `editStory` | `Story` | + |
| `exportChatInviteLink` | `String` | + |
| `forwardMessage` | `Message` | + |
| `forwardMessages` | `MessageId[]` | + |
| `getAvailableGifts` | `Gifts` | + |
| `getBusinessAccountGifts` | `OwnedGifts` | + |
| `getBusinessAccountStarBalance` | `Telegram` | + |
| `getBusinessConnection` | `BusinessConnection` | + |
| `getChat` | `ChatFullInfo` | + |
| `getChatAdministrators` | `ChatMember[]` | + |
| `getChatGifts` | `OwnedGifts` | + |
| `getChatMember` | `ChatMember` | + |
| `getChatMemberCount` | `Integer` | + |
| `getChatMenuButton` | `MenuButton` | + |
| `getFile` | `File` | + |
| `getForumTopicIconStickers` | `Sticker[]` | + |
| `getManagedBotAccessSettings` | `BotAccessSettings` | + |
| `getManagedBotToken` | `String` | + |
| `getMe` | `User` | + |
| `getMyCommands` | `BotCommand[]` | + |
| `getMyDefaultAdministratorRights` | `ChatAdministratorRights` | + |
| `getMyDescription` | `BotDescription` | + |
| `getMyName` | `BotName` | + |
| `getMyShortDescription` | `BotShortDescription` | + |
| `getUserChatBoosts` | `UserChatBoosts` | + |
| `getUserGifts` | `OwnedGifts` | + |
| `getUserPersonalChatMessages` | `Message[]` | + |
| `getUserProfileAudios` | `UserProfileAudios` | + |
| `getUserProfilePhotos` | `UserProfilePhotos` | + |
| `giftPremiumSubscription` | `Boolean` | + |
| `hideGeneralForumTopic` | `Boolean` | + |
| `leaveChat` | `Boolean` | + |
| `logOut` | `Boolean` | + |
| `pinChatMessage` | `Boolean` | + |
| `postStory` | `Story` | + |
| `promoteChatMember` | `Boolean` | + |
| `readBusinessMessage` | `Boolean` | + |
| `removeBusinessAccountProfilePhoto` | `Boolean` | + |
| `removeChatVerification` | `Boolean` | + |
| `removeMyProfilePhoto` | `Boolean` | + |
| `removeUserVerification` | `Boolean` | + |
| `reopenForumTopic` | `Boolean` | + |
| `reopenGeneralForumTopic` | `Boolean` | + |
| `replaceManagedBotToken` | `String` | + |
| `repostStory` | `Story` | + |
| `restrictChatMember` | `Boolean` | + |
| `revokeChatInviteLink` | `ChatInviteLink` | + |
| `savePreparedInlineMessage` | `PreparedInlineMessage` | + |
| `savePreparedKeyboardButton` | `PreparedKeyboardButton` | + |
| `sendAnimation` | `Message` | + |
| `sendAudio` | `Message` | + |
| `sendChatAction` | `Boolean` | + |
| `sendChatJoinRequestWebApp` | `Boolean` | + |
| `sendChecklist` | `Message` | + |
| `sendContact` | `Message` | + |
| `sendDice` | `Message` | + |
| `sendDocument` | `Message` | + |
| `sendGift` | `Boolean` | + |
| `sendLivePhoto` | `Message` | + |
| `sendLocation` | `Message` | + |
| `sendMediaGroup` | `Message[]` | + |
| `sendMessage` | `Message` | + |
| `sendMessageDraft` | `Boolean` | + |
| `sendPaidMedia` | `Message` | + |
| `sendPhoto` | `Message` | + |
| `sendPoll` | `Message` | + |
| `sendVenue` | `Message` | + |
| `sendVideo` | `Message` | + |
| `sendVideoNote` | `Message` | + |
| `sendVoice` | `Message` | + |
| `setBusinessAccountBio` | `Boolean` | + |
| `setBusinessAccountGiftSettings` | `Boolean` | + |
| `setBusinessAccountName` | `Boolean` | + |
| `setBusinessAccountProfilePhoto` | `Boolean` | + |
| `setBusinessAccountUsername` | `Boolean` | + |
| `setChatAdministratorCustomTitle` | `Boolean` | + |
| `setChatDescription` | `Boolean` | + |
| `setChatMemberTag` | `Boolean` | + |
| `setChatMenuButton` | `Boolean` | + |
| `setChatPermissions` | `Boolean` | + |
| `setChatPhoto` | `Boolean` | + |
| `setChatStickerSet` | `Use` | + |
| `setChatTitle` | `Boolean` | + |
| `setManagedBotAccessSettings` | `Boolean` | + |
| `setMessageReaction` | `Boolean` | + |
| `setMyCommands` | `Boolean` | + |
| `setMyDefaultAdministratorRights` | `Boolean` | + |
| `setMyDescription` | `Boolean` | + |
| `setMyName` | `Boolean` | + |
| `setMyProfilePhoto` | `Boolean` | + |
| `setMyShortDescription` | `Boolean` | + |
| `setUserEmojiStatus` | `Boolean` | + |
| `transferBusinessAccountStars` | `Boolean` | + |
| `transferGift` | `Boolean` | + |
| `unbanChatMember` | `The` | + |
| `unbanChatSenderChat` | `Boolean` | + |
| `unhideGeneralForumTopic` | `Boolean` | + |
| `unpinAllChatMessages` | `Boolean` | + |
| `unpinAllForumTopicMessages` | `Boolean` | + |
| `unpinAllGeneralForumTopicMessages` | `Boolean` | + |
| `unpinChatMessage` | `Boolean` | + |
| `upgradeGift` | `Boolean` | + |
| `verifyChat` | `Boolean` | + |
| `verifyUser` | `Boolean` | + |

## Games (3)

| Method | Returns | Status |
|--------|---------|--------|
| `getGameHighScores` | `GameHighScore[]` | + |
| `sendGame` | `Message` | + |
| `setGameScore` | `Boolean` | + |

## Getting Updates (4)

| Method | Returns | Status |
|--------|---------|--------|
| `deleteWebhook` | `Boolean` | + |
| `getUpdates` | `Update[]` | + |
| `getWebhookInfo` | `WebhookInfo` | + |
| `setWebhook` | `Boolean` | + |

## Inline Mode (1)

| Method | Returns | Status |
|--------|---------|--------|
| `answerInlineQuery` | `Boolean` | + |

## Payments (8)

| Method | Returns | Status |
|--------|---------|--------|
| `answerPreCheckoutQuery` | `Boolean` | + |
| `answerShippingQuery` | `Boolean` | + |
| `createInvoiceLink` | `String` | + |
| `editUserStarSubscription` | `Boolean` | + |
| `getMyStarBalance` | `StarAmount` | + |
| `getStarTransactions` | `Telegram` | + |
| `refundStarPayment` | `Boolean` | + |
| `sendInvoice` | `Message` | + |

## Rich Messages (2)

| Method | Returns | Status |
|--------|---------|--------|
| `sendRichMessage` | `Message` | + |
| `sendRichMessageDraft` | `Boolean` | + |

## Stickers (16)

| Method | Returns | Status |
|--------|---------|--------|
| `addStickerToSet` | `Boolean` | + |
| `createNewStickerSet` | `Boolean` | + |
| `deleteStickerFromSet` | `Boolean` | + |
| `deleteStickerSet` | `Boolean` | + |
| `getCustomEmojiStickers` | `Sticker[]` | + |
| `getStickerSet` | `StickerSet` | + |
| `replaceStickerInSet` | `Boolean` | + |
| `sendSticker` | `Message` | + |
| `setCustomEmojiStickerSetThumbnail` | `Boolean` | + |
| `setStickerEmojiList` | `Boolean` | + |
| `setStickerKeywords` | `Boolean` | + |
| `setStickerMaskPosition` | `Boolean` | + |
| `setStickerPositionInSet` | `Boolean` | + |
| `setStickerSetThumbnail` | `Boolean` | + |
| `setStickerSetTitle` | `Boolean` | + |
| `uploadStickerFile` | `File` | + |

## Telegram Passport (1)

| Method | Returns | Status |
|--------|---------|--------|
| `setPassportDataErrors` | `The` | + |

## Updating Messages (14)

| Method | Returns | Status |
|--------|---------|--------|
| `approveSuggestedPost` | `Boolean` | + |
| `declineSuggestedPost` | `Boolean` | + |
| `deleteAllMessageReactions` | `Boolean` | + |
| `deleteMessage` | `Boolean` | + |
| `deleteMessageReaction` | `Boolean` | + |
| `deleteMessages` | `Boolean` | + |
| `editMessageCaption` | `Boolean` | + |
| `editMessageChecklist` | `Message` | + |
| `editMessageLiveLocation` | `Boolean` | + |
| `editMessageMedia` | `Boolean` | + |
| `editMessageReplyMarkup` | `Boolean` | + |
| `editMessageText` | `Boolean` | + |
| `stopMessageLiveLocation` | `Boolean` | + |
| `stopPoll` | `Poll` | + |

