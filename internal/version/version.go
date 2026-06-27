// Package version holds the static build version of mtgo-bot-api.
//
// The version is a compile-time constant (not ldflags-injected) so that every
// binary reports the same, reproducible value. Bump it here on release.
package version

// Version is the mtgo-bot-api server version, in semver form.
const Version = "0.1.0"

// BotAPIVersion is the official Telegram Bot API spec version this server
// implements. Mirrors the `project(TelegramBotApi VERSION …)` line in the
// reference telegram-bot-api CMakeLists.txt; update it when rebasing onto a
// newer official release. The --version output reproduces the official
// "Bot API <version>" line verbatim.
const BotAPIVersion = "10.1"
