/**
 * Webhook receiver + auto-responder for mtgo-bot-api testing.
 *
 * Receives incoming webhook POSTs, logs them, AND replies to /start commands
 * by calling the Bot API back (sendMessage). This makes it a fully functional
 * webhook-mode bot — not just a passive logger.
 *
 * Run locally:  PORT=8080 API_ROOT=http://localhost:8081 BOT_TOKEN=<token> bun run receiver.ts
 * Behind TLS:   reverse-proxy 443 -> PORT (via Cloudflare tunnel), point setWebhook at
 *               https://test.sadiq.lol/
 *
 * Environment:
 *   PORT          listen port (default 8080)
 *   SECRET_TOKEN  optional; must match setWebhook secret_token
 *   API_ROOT      Bot API server root (default http://localhost:8081)
 *   BOT_TOKEN     bot token (required for auto-reply)
 */
const PORT = Number(process.env.PORT ?? 8080);
const SECRET = process.env.SECRET_TOKEN ?? "";
const API_ROOT = process.env.API_ROOT ?? "http://localhost:8081";
const BOT_TOKEN = process.env.BOT_TOKEN ?? "";

let count = 0;

const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    // Health check.
    if (req.method === "GET") {
      return new Response(
        `webhook receiver + responder OK (received ${count} updates)\n` +
        `API_ROOT=${API_ROOT}  auto-reply=${BOT_TOKEN ? "ON" : "OFF"}\n`
      );
    }
    if (req.method !== "POST") {
      return new Response("Method Not Allowed", { status: 405 });
    }

    // Secret-token verification.
    if (SECRET) {
      const tok = req.headers.get("x-telegram-bot-api-secret-token") ?? "";
      if (tok !== SECRET) {
        console.warn("⚠️  secret token mismatch — rejecting");
        return new Response("Forbidden", { status: 403 });
      }
    }

    count++;
    const body = await req.text();
    let update: any;
    try { update = JSON.parse(body); } catch { update = { raw: body }; }

    // Log the update.
    const keys = typeof update === "object" ? Object.keys(update).join(",") : "?";
    console.log(`📨 #${count} ${new Date().toISOString()} fields=[${keys}]`);

    // Auto-reply to messages.
    if (BOT_TOKEN && update.message) {
      const msg = update.message;
      const chatId = msg.chat?.id;
      const text = msg.text ?? "";

      if (text.startsWith("/start")) {
        await apiCall("sendMessage", {
          chat_id: chatId,
          text: "👋 Hello! I'm a webhook-mode bot running on mtgo-bot-api.\n" +
                "Send any message and I'll echo it back!",
          parse_mode: "HTML",
        });
      } else if (text.startsWith("/help")) {
        await apiCall("sendMessage", {
          chat_id: chatId,
          text: "Commands:\n/start — welcome\n/help — this message\n/ping — pong\n" +
                "Any other text — echo",
        });
      } else if (text.startsWith("/ping")) {
        await apiCall("sendMessage", { chat_id: chatId, text: "🏓 pong!" });
      } else if (text) {
        await apiCall("sendMessage", {
          chat_id: chatId,
          text: `📢 Echo: ${text}`,
          reply_to_message_id: msg.message_id,
        });
      }

      // Handle callback queries.
      if (update.callback_query) {
        const cq = update.callback_query;
        await apiCall("answerCallbackQuery", {
          callback_query_id: cq.id,
          text: `✅ Received: ${cq.data}`,
        });
      }
    }

    // Log the full update (truncated).
    console.log(JSON.stringify(update, null, 2).slice(0, 1000));

    return new Response("ok");
  },
});

/** Call a Bot API method via the local server. */
async function apiCall(method: string, params: Record<string, any>) {
  try {
    const res = await fetch(`${API_ROOT}/bot${BOT_TOKEN}/${method}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(params),
    });
    const json = await res.json();
    if (!json.ok) {
      console.error(`❌ ${method}: ${json.description}`);
    }
    return json;
  } catch (e: any) {
    console.error(`❌ ${method} fetch error: ${e.message}`);
  }
}

console.log(
  `🌐 webhook receiver + responder on :${PORT}\n` +
  `   secret: ${SECRET ? "ON" : "OFF"}\n` +
  `   API_ROOT: ${API_ROOT}\n` +
  `   auto-reply: ${BOT_TOKEN ? "ON" : "OFF (set BOT_TOKEN to enable)"}`
);
