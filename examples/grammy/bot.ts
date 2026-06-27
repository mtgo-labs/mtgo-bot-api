/**
 * grammY example pointing at a local mtgo-bot-api server.
 * (A minimal version of examples/test-bot/ — see that for the full comparison harness.)
 *
 * Run: bun install grammy && BOT_TOKEN=<token> bun run examples/grammy/bot.ts
 */
import { Bot, InlineKeyboard } from "grammy";

const TOKEN = process.env.BOT_TOKEN ?? "";
const API_ROOT = process.env.API_ROOT ?? "http://localhost:8081";

if (!TOKEN) {
  console.error("BOT_TOKEN env var is required");
  process.exit(1);
}

const bot = new Bot(TOKEN, { client: { apiRoot: API_ROOT } });

bot.command("start", (ctx) =>
  ctx.reply("👋 Hello from mtgo-bot-api via *grammY*!", { parse_mode: "Markdown" })
);

bot.command("test", (ctx) => ctx.reply("📦 grammY round-trip OK"));

bot.command("inline", async (ctx) => {
  const kb = new InlineKeyboard().text("Tap me", "tap");
  await ctx.reply("Inline keyboard via grammY:", { reply_markup: kb });
});

bot.callbackQuery("tap", (ctx) => ctx.answerCallbackQuery({ text: "grammY callback OK ✅" }));

// Auto-reconnect so the bot survives server restarts.
async function runForever() {
  while (true) {
    try {
      const me = await bot.api.getMe();
      console.log(`✅ grammY connected: @${me.username} (${me.id})  base=${API_ROOT}`);
      console.log("📡 grammY polling…");
      await bot.start();
    } catch (err: any) {
      console.error(`⚠️ ${err?.message ?? err}. Reconnecting in 3s…`);
      await new Promise((r) => setTimeout(r, 3000));
    }
  }
}

runForever();
