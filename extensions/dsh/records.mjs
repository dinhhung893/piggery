// dsh's session events as piggery's standard records (the pi-rpc shapes `piggery tail` and top read:
// message_end with usage, tool_execution_start/end, turn_end per model call, agent_end). Pure: the
// caller passes the events and a write. A worker's records go to its stdout (the driver's log); a
// web session's go to a file it reports as its transcript (format "driver"), since dsh's own
// session files are compressed and piggery does not read them.

// Message sources whose text is what the participant said or was told: the human's prompt and
// piggery's mail. Runtime context, skill catalogs and reminders are not part of the tail.
const SAID = new Set(["user", "piggery-mail"]);

// The log is compact: a text longer than this is cut (tail shows a line of it; the full text is in dsh's own session).
const MAX_TEXT = 2000;
const clip = (s) => (s.length > MAX_TEXT ? s.slice(0, MAX_TEXT) + "…" : s);
const texts = (content) => (content ?? []).filter((b) => b.type === "text" && b.text).map((b) => ({ type: "text", text: clip(b.text) }));

export class Records {
	/** @param {(record: object) => void} write */
	constructor(write) {
		this.write = write;
		this.tools = new Map(); // tool name by call id, for the result
	}

	/** One dsh SessionEvent ({type, data}). */
	event(ev) {
		const d = ev.data ?? {};
		switch (ev.type) {
			case "user/message": {
				if (!SAID.has(d.source?.kind)) return;
				const content = texts(d.content);
				if (content.length) this.write({ type: "message_end", message: { role: "user", content } });
				return;
			}
			case "assistant/message": {
				const message = { role: "assistant", content: texts(d.message?.content) };
				// Context in use, as pi's usage: the call's input (dsh's inputTokens leaves cache reads out),
				// output and cache together.
				const u = d.usage;
				if (u) {
					const [input, output, cacheRead, cacheWrite] = [u.inputTokens ?? 0, u.outputTokens ?? 0, u.cacheReadTokens ?? 0, u.cacheWriteTokens ?? 0];
					message.usage = { input, output, cacheRead, cacheWrite, totalTokens: input + output + cacheRead + cacheWrite };
				}
				this.write({ type: "message_end", message });
				return;
			}
			case "tool/call": {
				this.tools.set(d.callId, d.name);
				let args = {};
				try {
					args = JSON.parse(d.arguments ?? "{}");
				} catch {}
				this.write({ type: "tool_execution_start", toolName: d.name, args });
				return;
			}
			case "tool/result": {
				const m = d.message ?? {};
				const name = this.tools.get(m.toolCallId) ?? "";
				this.tools.delete(m.toolCallId);
				this.write({ type: "tool_execution_end", toolName: name, isError: m.isError === true, result: { content: texts(m.content) } });
				return;
			}
			case "step/end": // one model call
				this.write({ type: "turn_end" });
				return;
			case "turn/end": {
				const r = d.reason ?? {};
				if (r.kind === "error")
					this.write({ type: "message_end", message: { role: "assistant", stopReason: "error", errorMessage: r.error?.message ?? "error" } });
				this.write({ type: "agent_end" });
				return;
			}
		}
	}
}
