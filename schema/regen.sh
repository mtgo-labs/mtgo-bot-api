#!/usr/bin/env bash
# Regenerate the scrape-derived schema artifacts from the official Bot API docs.
#
# This is the single entry point for refreshing schema/*.json:
#   1. scrape   — fetch core.telegram.org/bots/api -> methods.json, types.json
#   2. validate — cross-check the schema against the live implementation
#   3. generate — write COVERAGE.md and METHODS.md
#
# The JSON files are machine-generated (see their _generated marker). Never
# hand-edit methods.json or types.json — edit status.json instead, or change the
# scraper and re-run this script.
#
# Usage:  ./schema/regen.sh
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> scraping official docs"
go run ./schema/cmd/scrape

echo "==> validating against implementation"
go run ./schema/cmd/validate || true   # parity gaps are informational, not fatal here

echo "==> generating reports"
go run ./schema/cmd/generate

echo "==> done. Review the diff in schema/ before committing."
