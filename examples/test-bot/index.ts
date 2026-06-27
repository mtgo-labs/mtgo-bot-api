/**
 * Comprehensive test bot for mtgo-bot-api.
 * Tests ~40 Bot API methods across all categories.
 *
 * Usage:
 *   TOKEN=<bot_token> bun run index.ts                 # local server (localhost:8081)
 *   # or put TOKEN=... (and optionally API_ROOT) in a .env file — Bun auto-loads it.
 *   API_ROOT=https://api.telegram.org TOKEN=<bot_token> bun run index.ts  # official server
 */

import { Bot, InlineKeyboard } from "grammy";

const TOKEN = process.env.TOKEN;
if (!TOKEN) {
  console.error("TOKEN environment variable is required. Get one from @BotFather and set it via env or a .env file.");
  process.exit(1);
}
const API_ROOT = process.env.API_ROOT ?? "http://localhost:8081";
const CHAT_ID = 1845033319; // test user

const bot = new Bot(TOKEN, {
  client: { apiRoot: API_ROOT },
});

console.log(`🔗 API root: ${API_ROOT}`);
console.log(`🤖 Token: ${TOKEN.slice(0, 10)}...`);

// ─── Helpers ───

function html(obj: unknown): string {
  return "<pre>" + JSON.stringify(obj, null, 2).slice(0, 4000) + "</pre>";
}

// ─── Comparison helpers (local vs official Bot API) ───

const OFFICIAL_ROOT = "https://api.telegram.org";

/** resultKeys returns a canonical signature of a Bot API result for comparison. */
function resultKeys(res: any): string {
  const r = res?.result;
  if (r === null || r === undefined) return "null";
  if (typeof r !== "object") return typeof r; // boolean, number, string
  if (Array.isArray(r)) {
    if (r.length === 0) return "list:empty";
    return `list[${r.length}]{${Object.keys(r[0]).sort().join(",")}}`;
  }
  return `{${Object.keys(r).sort().join(",")}}`;
}

/** compareMethod POSTs the same params to local + official and compares the result shape. */
async function compareMethod(label: string, method: string, params: Record<string, unknown>): Promise<string> {
  const body = JSON.stringify(params);
  const headers = { "Content-Type": "application/json" };
  const call = async (root: string) =>
    fetch(`${root}/bot${TOKEN}/${method}`, { method: "POST", headers, body })
      .then((r) => r.json())
      .catch((e: any) => ({ ok: false, description: e.message }));
  const [loc, off] = await Promise.all([call(API_ROOT), call(OFFICIAL_ROOT)]);
  const locOk = loc?.ok === true;
  const offOk = off?.ok === true;
  const desc = (j: any) => (j?.description ?? "unknown").toString().slice(0, 70);
  const tag = label.padEnd(15);
  if (!locOk && !offOk) return `⏭️  ${tag} both failed: ${desc(loc)}`;
  if (!locOk) return `❌ ${tag} local: ${desc(loc)} | official: ok`;
  if (!offOk) return `⚠️ ${tag} local: ok | official: ${desc(off)}`;
  const lk = resultKeys(loc);
  const fk = resultKeys(off);
  if (lk === fk) return `✅ ${tag} keys match`;
  return `⚠️ ${tag} key mismatch\n      local:    ${lk}\n      official: ${fk}`;
}

// ─── Core Methods ───

bot.command("start", async (ctx) => {
  await ctx.reply(
    "🧪 mtgo-bot-api test bot\n\n" +
    "── Core ──\n" +
    "/me /chat /user /raw\n\n" +
    "── Messaging ──\n" +
    "/send /edit /reply /forward /copy /delete\n\n" +
    "── Media ──\n" +
    "/photo /video /audio /voice /document /sticker /media /animation\n\n" +
    "── Interactive ──\n" +
    "/poll /quiz /dice /location /venue /contact /action\n\n" +
    "── Chat ──\n" +
    "/chatinfo /membercount /chatmember /pins\n\n" +
    "── Bot Profile ──\n" +
    "/mycommands /myname /mydesc\n\n" +
    "── Inline ──\n" +
    "/keyboard /inline\n\n" +
    "── Callback & Inline Mode ──\n" +
    "/callbacktest — diverse inline keyboard (click to test callbacks)\n" +
    "@<bot> <query> — inline mode results (enable via @BotFather)\n\n" +
    "── Rich Message (v10.1) ──\n" +
    "/rich /richhtml /richdraft\n\n" +
    "── Batch ──\n" +
    "/testall — compare local vs official Bot API\n" +
    "/drop — clear stale pending updates\n" +
    "/compare — compare getMe fields\n\n" +
    "── Webhook ──\n" +
    "/setwebhook <url> [secret] /delwebhook /webhookinfo\n" +
    "(setting a webhook disables long polling — stop this bot first)\n" +
    "/testall — run all tests\n"
  );
});

bot.command("me", async (ctx) => {
  const me = await bot.api.getMe();
  await ctx.reply(html(me), { parse_mode: "HTML" });
});

bot.command("chat", async (ctx) => {
  const chat = await bot.api.getChat(ctx.chat.id);
  await ctx.reply(html(chat), { parse_mode: "HTML" });
});

bot.command("user", async (ctx) => {
  await ctx.reply(html(ctx.from), { parse_mode: "HTML" });
});

bot.command("raw", async (ctx) => {
  await ctx.reply(html(ctx.msg), { parse_mode: "HTML" });
});

// ─── Messaging ───

bot.command("send", async (ctx) => {
  const msg = await ctx.reply("Hello from mtgo-bot-api! 🚀");
  await ctx.reply(`✅ sendMessage: message_id=${msg.message_id}`);
});

bot.command("edit", async (ctx) => {
  const msg = await ctx.reply("Original message");
  await new Promise(r => setTimeout(r, 1000));
  const edited = await ctx.api.editMessageText(ctx.chat.id, msg.message_id, "Edited message ✏️");
  await ctx.reply(`✅ editMessageText`);
});

bot.command("reply", async (ctx) => {
  const msg = await ctx.reply("Reply to this message!", { reply_parameters: { message_id: ctx.msg.message_id } });
  await ctx.reply(`✅ reply: message_id=${msg.message_id}`);
});

bot.command("forward", async (ctx) => {
  const msg = await ctx.forwardMessage(ctx.chat.id, ctx.chat.id, ctx.msg.message_id);
  await ctx.reply(`✅ forwardMessage: message_id=${msg.message_id}`);
});

bot.command("copy", async (ctx) => {
  const msg = await ctx.copyMessage(ctx.chat.id, ctx.chat.id, ctx.msg.message_id);
  await ctx.reply(`✅ copyMessage`);
});

bot.command("delete", async (ctx) => {
  const msg = await ctx.reply("This will be deleted in 2s...");
  await new Promise(r => setTimeout(r, 2000));
  await ctx.api.deleteMessage(ctx.chat.id, msg.message_id);
  await ctx.reply(`✅ deleteMessage`);
});

// ─── Media ───

bot.command("photo", async (ctx) => {
  const msg = await ctx.replyWithPhoto("https://telegram.org/img/t_logo.png", { caption: "Telegram logo 📸" });
  await ctx.reply(`✅ sendPhoto: message_id=${msg.message_id}`);
});

bot.command("video", async (ctx) => {
  await ctx.replyWithVideo("https://www.w3schools.com/html/mov_bbb.mp4", { caption: "Test video 🎬" });
  await ctx.reply("✅ sendVideo");
});

bot.command("audio", async (ctx) => {
  await ctx.replyWithAudio("https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3", {
    title: "Test Song",
    performer: "SoundHelix",
  });
  await ctx.reply("✅ sendAudio");
});

bot.command("voice", async (ctx) => {
  await ctx.replyWithVoice("https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3");
  await ctx.reply("✅ sendVoice");
});

bot.command("document", async (ctx) => {
  await ctx.replyWithDocument("https://www.w3schools.com/html/mov_bbb.mp4", { caption: "Test document 📄" });
  await ctx.reply("✅ sendDocument");
});

bot.command("sticker", async (ctx) => {
  // Use a known sticker file_id or URL
  await ctx.replyWithSticker("CAACAgIAAxkBAAELGpZl2Vz...");
  await ctx.reply("✅ sendSticker");
});

bot.command("animation", async (ctx) => {
  await ctx.replyWithAnimation("https://media.giphy.com/media/3o7TKMt1VVNkHV2PaE/giphy.gif", { caption: "GIF 🎞️" });
  await ctx.reply("✅ sendAnimation");
});

bot.command("media", async (ctx) => {
  await ctx.replyWithMediaGroup([
    { type: "photo", media: "https://telegram.org/img/t_logo.png" },
    { type: "photo", media: "https://telegram.org/img/t_logo.png" },
  ]);
  await ctx.reply("✅ sendMediaGroup");
});

// ─── Interactive ───

bot.command("poll", async (ctx) => {
  await ctx.replyWithPoll("What's your favorite language?", ["Go", "TypeScript", "Python", "Rust"], {
    is_anonymous: false,
  });
  await ctx.reply("✅ sendPoll");
});

bot.command("quiz", async (ctx) => {
  await ctx.replyWithPoll("What is 2+2?", ["3", "4", "5"], {
    type: "quiz",
    correct_option_id: 1,
    is_anonymous: false,
  });
  await ctx.reply("✅ sendPoll (quiz)");
});

bot.command("dice", async (ctx) => {
  const msg = await ctx.replyWithDice();
  await ctx.reply(`✅ sendDice: value=${msg.dice?.value}`);
});

bot.command("location", async (ctx) => {
  await ctx.replyWithLocation(37.7749, -122.4194, { live_period: 60 });
  await ctx.reply("✅ sendLocation");
});

bot.command("venue", async (ctx) => {
  await ctx.replyWithVenue(37.7749, -122.4194, "Test Venue", "123 Main St");
  await ctx.reply("✅ sendVenue");
});

bot.command("contact", async (ctx) => {
  await ctx.replyWithContact("+1234567890", "Test", "Contact");
  await ctx.reply("✅ sendContact");
});

bot.command("action", async (ctx) => {
  await ctx.replyWithChatAction("typing");
  await ctx.reply("✅ sendChatAction");
});

// ─── Chat Info ───

bot.command("chatinfo", async (ctx) => {
  const chat = await bot.api.getChat(ctx.chat.id);
  await ctx.reply(html(chat), { parse_mode: "HTML" });
});

bot.command("membercount", async (ctx) => {
  const count = await bot.api.getChatMemberCount(ctx.chat.id);
  await ctx.reply(`✅ getChatMemberCount: ${count}`);
});

bot.command("chatmember", async (ctx) => {
  const member = await bot.api.getChatMember(ctx.chat.id, ctx.from.id);
  await ctx.reply(html(member), { parse_mode: "HTML" });
});

bot.command("pins", async (ctx) => {
  const msg = await ctx.reply("Pin this message...");
  try {
    await ctx.api.pinChatMessage(ctx.chat.id, msg.message_id);
    await ctx.api.unpinChatMessage(ctx.chat.id, msg.message_id);
    await ctx.reply("✅ pinChatMessage + unpinChatMessage");
  } catch (e: any) {
    await ctx.reply(`⚠️ pin/unpin: ${e.message?.slice(0, 80)}`);
  }
});

// ─── Bot Profile ───

bot.command("mycommands", async (ctx) => {
  const cmds = await bot.api.getMyCommands();
  await ctx.reply(html(cmds), { parse_mode: "HTML" });
});

bot.command("myname", async (ctx) => {
  const name = await bot.api.getMyName();
  await ctx.reply(html(name), { parse_mode: "HTML" });
});

bot.command("mydesc", async (ctx) => {
  const desc = await bot.api.getMyDescription();
  await ctx.reply(html(desc), { parse_mode: "HTML" });
});

// ─── Inline Keyboard ───

bot.command("keyboard", async (ctx) => {
  const kb = new InlineKeyboard()
    .text("✅ Yes", "yes")
    .text("❌ No", "no")
    .row()
    .url("🔗 GitHub", "https://github.com/mtgo-labs/mtgo-bot-api");
  await ctx.reply("Inline keyboard test:", { reply_markup: kb });
});

bot.callbackQuery("yes", async (ctx) => {
  await ctx.answerCallbackQuery({ text: "You said Yes!" });
  await ctx.editMessageText("✅ You chose: Yes");
});

bot.callbackQuery("no", async (ctx) => {
  await ctx.answerCallbackQuery({ text: "You said No!" });
  await ctx.editMessageText("❌ You chose: No");
});

// ─── Drop pending updates (clear stale backlog) ───
// Inline queries expire after ~15s. After server rebuilds/crashes, the
// persisted TQueue holds expired inline_query updates that always fail with
// "query is too old". /drop calls deleteWebhook(drop_pending_updates=true) to
// clear the queue so only fresh updates are delivered.

bot.command("drop", async (ctx) => {
  try {
    const res = await fetch(`${API_ROOT}/bot${TOKEN}/deleteWebhook?drop_pending_updates=true`).then(r => r.json());
    await ctx.reply(res.ok ? "🗑️ Pending updates cleared." : `❌ ${res.description}`);
  } catch (e: any) {
    await ctx.reply(`❌ ${e.message}`);
  }
});

// ─── Webhook ───
// /setwebhook <url> [secret]   — point the server at a webhook receiver
// /delwebhook                  — stop webhook delivery, switch back to polling
// /webhookinfo                 — show current webhook status
// Example: /setwebhook https://test.sadiq.lol/ mysecret

bot.command("setwebhook", async (ctx) => {
  const parts = (ctx.message?.text ?? "").split(/\s+/).slice(1);
  const url = parts[0];
  if (!url) {
    await ctx.reply("Usage: /setwebhook <url> [secret_token]");
    return;
  }
  const secret = parts[1] ?? "";
  const params: Record<string, string> = { url };
  if (secret) params.secret_token = secret;
  try {
    const res = await fetch(`${API_ROOT}/bot${TOKEN}/setWebhook`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(params),
    }).then(r => r.json());
    await ctx.reply(res.ok ? `✅ Webhook set → ${url}${secret ? " (secret ON)" : ""}` : `❌ ${res.description}`);
  } catch (e: any) {
    await ctx.reply(`❌ ${e.message}`);
  }
});

bot.command("delwebhook", async (ctx) => {
  try {
    const res = await fetch(`${API_ROOT}/bot${TOKEN}/deleteWebhook`).then(r => r.json());
    await ctx.reply(res.ok ? "🗑️ Webhook deleted (back to polling)." : `❌ ${res.description}`);
  } catch (e: any) {
    await ctx.reply(`❌ ${e.message}`);
  }
});

bot.command("webhookinfo", async (ctx) => {
  try {
    const res = await fetch(`${API_ROOT}/bot${TOKEN}/getWebhookInfo`).then(r => r.json());
    await ctx.reply(html(res), { parse_mode: "HTML" });
  } catch (e: any) {
    await ctx.reply(`❌ ${e.message}`);
  }
});

// ─── Callback Query (comprehensive) ───

bot.command("callbacktest", async (ctx) => {
  // A diverse inline keyboard exercising every button type the Bot API
  // supports: callback_data, url, switch_inline_query, switch_inline_query_current_chat,
  // copy_text, and login (url variant). Click any callback button to test the
  // callback flow; the handler below answers + edits the message.
  const kb = new InlineKeyboard()
    .text("🔵 Click A", "cb:a")
    .text("🟢 Click B", "cb:b")
    .row()
    .text("⚠️ Alert popup", "cb:alert")
    .text("🎧 Toast", "cb:toast")
    .row()
    .url("🔗 URL button", "https://core.telegram.org/bots/api")
    .row()
    .switchInline("🔁 Switch inline", "query ")
    .row()
    .switchInlineCurrent("💬 Switch inline (this chat)", "hello ")
    .row()
    // copy_text requires Bot API 7.x+; harmless if unsupported.
    .text("📋 Copy text", "cb:copy");
  await ctx.reply("🧪 Callback test — click a button:", { reply_markup: kb });
});

// Single callback handler for the "cb:*" data patterns above.
bot.callbackQuery(/^cb:/, async (ctx) => {
  const action = ctx.callbackQuery.data.slice(3);
  switch (action) {
    case "a":
      await ctx.answerCallbackQuery({ text: "A clicked ✅" });
      await ctx.editMessageText("You clicked A.");
      break;
    case "b":
      await ctx.answerCallbackQuery({ text: "B clicked ✅" });
      await ctx.editMessageText("You clicked B.");
      break;
    case "alert":
      // showAlert forces a modal popup (instead of a toast).
      await ctx.answerCallbackQuery({ text: "This is a modal alert!", show_alert: true });
      break;
    case "toast":
      await ctx.answerCallbackQuery({ text: "Quick toast 👋" });
      break;
    case "copy":
      // copy_text is set on the button itself; here we just acknowledge.
      await ctx.answerCallbackQuery({ text: "Use the button to copy text." });
      break;
    default:
      await ctx.answerCallbackQuery({ text: `Unknown: ${action}` });
  }
});

// ─── Inline Query (inline mode) ───
// Test by @mentioning the bot in any chat and typing a query. Enable inline
// mode for the bot via @BotFather → Bot Settings → Inline Mode.

bot.inlineQuery(/.*/, async (ctx) => {
  const q = ctx.inlineQuery.query.trim();
  // A small gallery of result types so the user can verify inline rendering.
  const results: any[] = [
    {
      type: "article",
      id: "1",
      title: `Echo: ${q || "(empty)"}`,
      description: "Sends your query back as text",
      input_message_content: { message_text: `You searched: ${q}` },
    },
    {
      type: "article",
      id: "2",
      title: "Bold message",
      description: "HTML-formatted result",
      input_message_content: { message_text: "<b>Bold</b> <i>inline</i> result", parse_mode: "HTML" },
    },
    {
      type: "article",
      id: "3",
      title: "MarkdownV2 result",
      description: "MarkdownV2-formatted result",
      input_message_content: { message_text: "*bold* _italic_", parse_mode: "MarkdownV2" },
    },
  ];
  // A photo + gif result to exercise media inline rendering.
  results.push(
    {
      type: "photo",
      id: "4",
      photo_url: "https://telegram.org/img/t_logo.png",
      thumbnail_url: "https://telegram.org/img/t_logo.png",
      caption: "Inline photo result",
    },
    {
      type: "gif",
      id: "5",
      gif_url: "https://media.giphy.com/media/3o7TKMt1VVNkHV2PaE/giphy.gif",
      thumbnail_url: "https://media.giphy.com/media/3o7TKMt1VVNkHV2PaE/giphy.gif",
      caption: "Inline GIF result",
    },
  );
  await ctx.answerInlineQuery(results, { cache_time: 0 });
});

// ─── Chosen Inline Result ───
// Fires when a user SELECTS an inline result from the results panel and sends
// it. For results sent via inline mode (to a chat the bot isn't in), the update
// carries an inline_message_id we can edit. Either way we log the choice.

bot.on("chosen_inline_result", async (ctx) => {
  const r = ctx.chosenInlineResult;
  console.log(
    `✅ ChosenInlineResult: result_id=${r.result_id} query="${r.query}" ` +
    `from=@${r.from.username ?? r.from.first_name} inline_message_id=${r.inline_message_id ?? "(none)"}`
  );
  // If the result was sent via inline mode, edit the sent message via raw API
  // (uses inline_message_id in place of chat_id + message_id).
  if (r.inline_message_id) {
    try {
      await fetch(`${API_ROOT}/bot${TOKEN}/editMessageText`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          inline_message_id: r.inline_message_id,
          text: `✅ You chose result <b>${r.result_id}</b> for "<i>${r.query}</i>"`,
          parse_mode: "HTML",
        }),
      });
    } catch {
      // Edit may fail if the message can't be modified; ignore.
    }
  }
});

// ─── Rich Message (Bot API v10.1) ───

bot.command("rich", async (ctx) => {
  const richMessage = {
    markdown: "**Bold** and _italic_ with [link](https://example.com)\n\n```go\nfmt.Println(\"Hello\")\n```",
    is_rtl: false,
  };
  try {
    const result = await fetch(`${API_ROOT}/bot${TOKEN}/sendRichMessage`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ chat_id: ctx.chat.id, rich_message: JSON.stringify(richMessage) }),
    }).then(r => r.json());
    await ctx.reply(html(result), { parse_mode: "HTML" });
  } catch (e: any) {
    await ctx.reply(`❌ sendRichMessage: ${e.message}`);
  }
});

bot.command("richhtml", async (ctx) => {
  const richMessage = {
    html: "<b>Bold</b> and <i>italic</i> with <a href='https://example.com'>link</a>",
    is_rtl: false,
  };
  try {
    const result = await fetch(`${API_ROOT}/bot${TOKEN}/sendRichMessage`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ chat_id: ctx.chat.id, rich_message: JSON.stringify(richMessage) }),
    }).then(r => r.json());
    await ctx.reply(html(result), { parse_mode: "HTML" });
  } catch (e: any) {
    await ctx.reply(`❌ sendRichMessage HTML: ${e.message}`);
  }
});

bot.command("richdraft", async (ctx) => {
  const richMessage = { markdown: "This is a **draft** rich message", is_rtl: false };
  try {
    const result = await fetch(`${API_ROOT}/bot${TOKEN}/sendRichMessageDraft`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ chat_id: ctx.chat.id, rich_message: JSON.stringify(richMessage) }),
    }).then(r => r.json());
    await ctx.reply(html(result), { parse_mode: "HTML" });
  } catch (e: any) {
    await ctx.reply(`❌ sendRichMessageDraft: ${e.message}`);
  }
});

// ─── Batch Test (local vs official Bot API comparison) ───

bot.command("testall", async (ctx) => {
  await ctx.reply("🧪 Comparing local vs official Bot API...");
  const cid = ctx.chat.id;
  const uid = ctx.from.id;

  // Each entry: [label, method, params]. POSTed to both endpoints identically.
  const tests: [string, string, Record<string, unknown>][] = [
    // ── Read-only (pure comparison) ──
    ["getMe", "getMe", {}],
    ["getChat", "getChat", { chat_id: cid }],
    ["getChatMemberCount", "getChatMemberCount", { chat_id: cid }],
    ["getChatMember", "getChatMember", { chat_id: cid, user_id: uid }],
    ["getMyCommands", "getMyCommands", {}],
    ["getMyName", "getMyName", {}],
    ["getMyDescription", "getMyDescription", {}],
    ["getWebhookInfo", "getWebhookInfo", {}],
    ["getMyShortDescription", "getMyShortDescription", {}],

    // ── Send methods (compare response shape; creates a message on each side) ──
    ["sendMessage", "sendMessage", { chat_id: cid, text: "compare_test" }],
    ["sendMessage HTML", "sendMessage", { chat_id: cid, text: "<b>bold</b> <i>italic</i>", parse_mode: "HTML" }],
    ["sendMessage MDv2", "sendMessage", { chat_id: cid, text: "*bold* _italic_", parse_mode: "MarkdownV2" }],
    ["sendPhoto", "sendPhoto", { chat_id: cid, photo: "https://telegram.org/img/t_logo.png" }],
    ["sendDocument", "sendDocument", { chat_id: cid, document: "https://telegram.org/img/t_logo.png" }],
    ["sendAnimation", "sendAnimation", { chat_id: cid, animation: "https://media.giphy.com/media/3o7TKMt1VVNkHV2PaE/giphy.gif" }],
    ["sendVideo", "sendVideo", { chat_id: cid, video: "https://www.w3schools.com/html/mov_bbb.mp4" }],
    ["sendAudio", "sendAudio", { chat_id: cid, audio: "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3" }],
    ["sendVoice", "sendVoice", { chat_id: cid, voice: "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3" }],
    ["sendPoll", "sendPoll", { chat_id: cid, question: "Test?", options: ["A", "B"], is_anonymous: false }],
    ["sendDice", "sendDice", { chat_id: cid }],
    ["sendLocation", "sendLocation", { chat_id: cid, latitude: 0, longitude: 0 }],
    ["sendContact", "sendContact", { chat_id: cid, phone_number: "123", first_name: "Test" }],
    ["sendChatAction", "sendChatAction", { chat_id: cid, action: "typing" }],

    // ── Rich Message (Bot API v10.1) ──
    // sendRichMessageDraft maps to messages.saveDraft with rich_message; returns Ok (true).
    ["sendRichMessage md", "sendRichMessage", { chat_id: cid, rich_message: JSON.stringify({ markdown: "**bold** _italic_ with [link](https://example.com)" }) }],
    ["sendRichMessage html", "sendRichMessage", { chat_id: cid, rich_message: JSON.stringify({ html: "<b>bold</b> and <i>italic</i>" }) }],
    ["sendRichMessageDraft", "sendRichMessageDraft", { chat_id: cid, draft_id: Date.now(), rich_message: JSON.stringify({ markdown: "draft rich message" }) }],
  ];

  const lines: string[] = [];
  for (const [label, method, params] of tests) {
    lines.push(await compareMethod(label, method, params));
  }

  const match = lines.filter((l) => l.startsWith("✅")).length;
  const warn = lines.filter((l) => l.startsWith("⚠️")).length;
  const fail = lines.filter((l) => l.startsWith("❌")).length;
  const skip = lines.filter((l) => l.startsWith("⏭️")).length;
  const header =
    `🧪 Local vs Official Bot API\n` +
    `Local: ${API_ROOT}\n` +
    `✅ ${match} match | ⚠️ ${warn} differ | ❌ ${fail} local-only fail | ⏭️ ${skip} both fail\n`;
  await ctx.reply((header + "\n" + lines.join("\n")).slice(0, 4000));
});

// ─── Compare ───

bot.command("compare", async (ctx) => {
  await ctx.reply("⏳ Comparing getMe...");
  try {
    const localMe = await fetch(`${API_ROOT}/bot${TOKEN}/getMe`).then(r => r.json());
    const officialMe = await fetch(`https://api.telegram.org/bot${TOKEN}/getMe`).then(r => r.json());
    const localKeys = Object.keys(localMe.result ?? {}).sort();
    const officialKeys = Object.keys(officialMe.result ?? {}).sort();
    let report = "📊 getMe comparison:\n\n";
    const missing = officialKeys.filter(k => !localKeys.includes(k));
    const extra = localKeys.filter(k => !officialKeys.includes(k));
    report += `Official: ${officialKeys.length} fields | Local: ${localKeys.length} fields\n\n`;
    if (missing.length) report += `❌ Missing:\n${missing.map(k => `  - ${k}`).join("\n")}\n\n`;
    if (extra.length) report += `➕ Extra:\n${extra.map(k => `  - ${k}`).join("\n")}\n\n`;
    if (!missing.length && !extra.length) report += "✅ Field names match!\n";
    await ctx.reply(report.slice(0, 4000));
  } catch (err: any) {
    await ctx.reply("❌ Failed: " + err.message);
  }
});

// ─── Echo ───

bot.on("message:text", async (ctx) => {
  if (ctx.message.text.startsWith("/")) return;
  await ctx.reply(
    `📝 Received: "${ctx.message.text}"\n` +
    `  from: ${ctx.from.first_name} (@${ctx.from.username})\n` +
    `  chat: ${ctx.chat.type} (${ctx.chat.id})`
  );
});

// ─── Error handler ───

bot.catch((err) => {
  console.error("Bot error:", err.error);
});

// ─── Start (with auto-reconnect) ───
// grammY's bot.start() exits when the long-poll connection breaks fatally
// (e.g. mtgo-bot-api restart killing the in-flight getUpdates). Wrap it in a
// retry loop so the test-bot survives server restarts and reconnects once the
// server is back up.

async function runForever() {
  while (true) {
    try {
      await bot.start({
        onStart: (info) => {
          console.log(`✅ Bot started: @${info.username} (${info.id})`);
          console.log(`📡 Polling for updates...`);
        },
      });
    } catch (err: any) {
      console.error(`⚠️ Polling stopped (${err?.message ?? err}). Reconnecting in 3s...`);
      await new Promise((r) => setTimeout(r, 3000));
    }
  }
}

runForever();
