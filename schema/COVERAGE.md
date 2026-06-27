# Bot API Coverage Report

- **Schema version:** 10.1
- **Implementation version:** 10.1
- **Schema methods:** 180 (official 180)
- **Implemented:** 180
- **Official coverage:** 180/180 (100.0%)

## Missing official methods (0)

_None — all official schema methods are registered._

## Untracked methods (registered, not in schema) (5)

- `answercustomquery`
- `getchatmemberscount`
- `kickchatmember`
- `sendcustomrequest`
- `setstickersetthumb`

## Incomplete parameter sets (0)

_None._

## Status overrides

- `close` — **implemented**: mtgo extension; mirrors TDLib close.
- `getchatmemberscount` — **implemented**: Deprecated alias of getChatMemberCount; kept for compatibility.
- `kickchatmember` — **implemented**: Deprecated alias of banChatMember; kept for compatibility.
- `logout` — **implemented**: mtgo extension; mirrors TDLib logout.
