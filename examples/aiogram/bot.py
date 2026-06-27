"""aiogram example pointing at a local mtgo-bot-api server.

Run:  pip install aiogram
      BOT_TOKEN=<token> python examples/aiogram/bot.py
"""

import asyncio
import os
import sys

from aiogram import Bot, Dispatcher, F
from aiogram.client.default import DefaultBotProperties
from aiogram.types import Message
from aiogram.enums import ParseMode

# aiogram v3: set the base API URL via the session's base.
from aiogram.utils.token import (
    TokenValidationError,
)  # noqa: F401  (ensures import path exists)

API_ROOT = "http://localhost:8081"
token = os.environ.get("BOT_TOKEN")
if not token:
    sys.exit("BOT_TOKEN env var is required")

# aiogram v3 Bot accepts a `base` (aiohttp session) — the simplest override is
# building the Bot with a custom session whose base URL is the local server.
bot = Bot(
    token=token,
    default=DefaultBotProperties(parse_mode=ParseMode.MARKDOWN),
    session=None,  # set below
)

# Override the API base URL: aiogram builds "<base>/bot<token>/<method>".
bot.session.api_root = API_ROOT

dp = Dispatcher()


@dp.message(F.text == "/start")
async def cmd_start(message: Message) -> None:
    await message.answer("👋 Hello from mtgo-bot-api via *aiogram*!")


@dp.message(F.text == "/test")
async def cmd_test(message: Message) -> None:
    await message.answer("📦 aiogram round-trip OK")


@dp.message(F.text == "/inline")
async def cmd_inline(message: Message) -> None:
    from aiogram.utils.keyboard import InlineKeyboardBuilder

    kb = InlineKeyboardBuilder().button(text="Tap me", callback_data="tap").as_markup()
    await message.answer("Inline keyboard via aiogram:", reply_markup=kb)


@dp.callback_query(F.data == "tap")
async def on_tap(call) -> None:
    await call.answer("aiogram callback OK ✅")


async def main() -> None:
    me = await bot.get_me()
    print(
        f"✅ aiogram connected: @{me.username} ({me.id})  base={bot.session.api_root}"
    )
    print("📡 aiogram polling…")
    await dp.start_polling(bot)


if __name__ == "__main__":
    asyncio.run(main())
