// Package storage implements SQLite-backed persistence for the TQueue, webhook
// config, and peer cache. It reimplements tdlib binlog log-semantics over
// modernc.org/sqlite (Constitution Principle 4). Per-bot file: <dir>/<token>/bot.db.
package storage
