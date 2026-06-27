// Package client implements the per-bot Client: all Bot API method handlers,
// update ingestion, and webhook setup. All Telegram calls go through the raw
// *tg.RPCClient (Constitution Principle 2). Mirrors telegram-bot-api/Client.cpp.
package client
