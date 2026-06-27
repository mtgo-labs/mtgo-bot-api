// Package version holds the build version of mtgo-bot-api.
//
// Version defaults to "0.1.0" for local and CI builds. Release builds override
// it to the git tag at link time via -ldflags
// "-X github.com/mtgo-labs/mtgo-bot-api/internal/version.Version=<tag>"
// (see .github/workflows/release.yml).
package version

// Version is the mtgo-bot-api server version, in semver form. A var (not const)
// so release builds can inject the tag via -ldflags.
var Version = "0.1.0"

// BotAPIVersion is the official Telegram Bot API spec version this server
// implements. Mirrors the `project(TelegramBotApi VERSION …)` line in the
// reference telegram-bot-api CMakeLists.txt; update it when rebasing onto a
// newer official release. The --version output reproduces the official
// "Bot API <version>" line verbatim.
const BotAPIVersion = "10.1"
