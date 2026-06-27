"""pyTelegramBotAPI (eternnoir/pyTelegramBotAPI) example pointing at a local
mtgo-bot-api server.

Run:  pip install pyTelegramBotAPI
      BOT_TOKEN=<token> python examples/pytelegrambotapi/bot.py
"""

import os
import sys

import telebot
from telebot import apihelper

# Point pyTelegramBotAPI at the local server. The template is
# "<root>/bot{token}/{method}". This overrides the default api.telegram.org.
apihelper.API_URL = "http://localhost:8081/bot{0}/{1}"

token = os.environ.get("BOT_TOKEN")
if not token:
    sys.exit("BOT_TOKEN env var is required")

bot = telebot.TeleBot(token)

print(f"✅ pyTelegramBotAPI configured: API_URL={apihelper.API_URL}")
me = bot.get_me()
print(f"🤖 getMe: @{me.username} ({me.id})")


@bot.message_handler(commands=["start"])
def cmd_start(message):
    bot.reply_to(
        message,
        "👋 Hello from mtgo-bot-api via *pyTelegramBotAPI*!",
        parse_mode="Markdown",
    )


@bot.message_handler(commands=["test"])
def cmd_test(message):
    bot.reply_to(message, "📦 pyTelegramBotAPI round-trip OK")


@bot.callback_query_handler(func=lambda c: True)
def on_callback(call):
    bot.answer_callback_query(call.id, text=f"Clicked: {call.data}")


print("📡 pyTelegramBotAPI polling…")
bot.infinity_polling()
