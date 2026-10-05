// Probe: logs every extension event (and a ctx snapshot) to $PROBE_LOG as JSONL, and can inject mail
// at session_stop. PROBE_INJECT = none | block | continue | steer | followup | aside;
// PROBE_MAILS = "text1|text2|..." (one is taken per session_stop; PROBE_ALL=1 sends all at once).
import { appendFileSync } from "node:fs";

const LOG = process.env.PROBE_LOG ?? "/tmp/pg-omp/probe.jsonl";
const mode = process.env.PROBE_INJECT ?? "none";
const mails = (process.env.PROBE_MAILS ?? "").split("|").filter(Boolean);
const t0 = Date.now();
let seq = 0;

const short = (v: unknown, d = 0): unknown => {
	if (typeof v === "string") return v.length > 120 ? v.slice(0, 120) + "…" : v;
	if (v === null || typeof v !== "object") return v;
	if (d >= 2) return Array.isArray(v) ? `[${v.length}]` : "{…}";
	if (Array.isArray(v)) return v.slice(0, 3).map((x) => short(x, d + 1));
	const o: Record<string, unknown> = {};
	for (const [k, x] of Object.entries(v as object)) o[k] = typeof x === "function" ? "fn" : short(x, d + 1);
	return o;
};

const log = (rec: Record<string, unknown>) =>
	appendFileSync(LOG, JSON.stringify({ n: ++seq, ms: Date.now() - t0, ...rec }) + "\n");

// Events in omp's ExtensionAPI, plus the ones pi has and omp is said to lack (they must never fire).
const OMP = ["resources_discover", "session_start", "session_switch", "session_branch", "session_compact", "session_shutdown",
	"session_before_tree", "session_tree", "context", "after_provider_response", "before_agent_start", "agent_start", "agent_end",
	"session_stop", "turn_start", "turn_end", "message_start", "message_update", "message_end", "tool_execution_start",
	"tool_execution_update", "tool_execution_end", "auto_compaction_start", "auto_compaction_end", "auto_retry_start",
	"auto_retry_end", "retry_fallback_applied", "retry_fallback_succeeded", "ttsr_triggered", "todo_reminder", "goal_updated",
	"input", "tool_call", "tool_result", "tool_approval_requested", "tool_approval_resolved"];
const PI_ONLY = ["agent_before_settle", "agent_settled", "ui_prompt_start", "ui_prompt_end", "model_select", "thinking_level_select"];

const brief = (name: string, e: any): Record<string, unknown> => {
	switch (name) {
		case "agent_end": { const m = e.messages ?? []; const l = m[m.length - 1]; return { willContinue: e.willContinue, messages: m.length,
			lastRole: l?.role, lastStop: l?.stopReason, lastErr: l?.isError, lastText: short(JSON.stringify(l?.content ?? "").slice(0, 80)),
			roles: m.map((x: any) => x.role + (x.stopReason ? ":" + x.stopReason : "")).join(",") }; }
		case "session_stop": return { stop_hook_active: e.stop_hook_active, turn_id: e.turn_id, messages: e.messages?.length,
			last: short(JSON.stringify(e.last_assistant_message?.content ?? "").slice(0, 100)) };
		case "message_start": case "message_end": return { role: e.message?.role, customType: e.message?.customType,
			display: e.message?.display, stopReason: e.message?.stopReason, text: short(JSON.stringify(e.message?.content ?? "").slice(0, 100)) };
		case "message_update": return { ame: e.assistantMessageEvent?.type };
		case "turn_start": case "turn_end": return { turnIndex: e.turnIndex, toolResults: e.toolResults?.length };
		case "tool_execution_start": case "tool_call": return { tool: e.toolName, args: short(e.args ?? e.input) };
		case "tool_execution_end": case "tool_result": return { tool: e.toolName, isError: e.isError };
		case "auto_retry_start": return short(e) as any;
		case "input": return { text: short(e.text) };
		case "context": return { messages: e.messages?.length };
		default: return short(e) as any;
	}
};

export default function probe(pi: any) {
	log({ ev: "_loaded", apiKeys: Object.keys(pi).sort().slice(0, 60) });
	for (const name of [...OMP, ...PI_ONLY]) {
		try {
			pi.on(name, async (e: any, ctx: any) => {
				if (name === "message_update" && !["start", "done", "error"].includes(e.assistantMessageEvent?.type)) return;
				let snap: Record<string, unknown> = {};
				try {
					snap = { idle: ctx.isIdle?.(), pending: ctx.hasPendingMessages?.(), model: ctx.model ? `${ctx.model.provider}/${ctx.model.id}` : null,
						thinking: pi.getThinkingLevel?.() };
				} catch (err: any) { snap = { snapErr: String(err?.message ?? err) }; }
				log({ ev: name, ...brief(name, e), ctx: snap });
				if (name === "before_agent_start" && process.env.PROBE_SYS) {
					log({ ev: "_sys", parts: e.systemPrompt?.length, appended: true });
					return { systemPrompt: [...(e.systemPrompt ?? []), process.env.PROBE_SYS] };
				}
				if (name === "session_start") log({ ev: "_ctx", mode: ctx.mode, hasUI: ctx.hasUI, cwd: ctx.cwd, sessionId: ctx.sessionManager?.getSessionId?.(),
					sessionFile: ctx.sessionManager?.getSessionFile?.(), agent: short(ctx.agent) });
				if (name === "session_stop" && mails.length && mode !== "none") {
					const take = process.env.PROBE_ALL === "1" ? mails.splice(0) : [mails.shift()!];
					const text = take.join("\n---\n");
					log({ ev: "_inject", how: mode, text: short(text) });
					if (mode === "block") return { decision: "block", reason: text };
					if (mode === "continue") return { continue: true, additionalContext: text };
					if (mode === "steer") { for (const t of take) pi.sendUserMessage(t, { deliverAs: "steer" }); return; }
					if (mode === "followup") { for (const t of take) pi.sendUserMessage(t, { deliverAs: "followUp" }); return; }
					if (mode === "aside") { for (const t of take) pi.sendUserMessage(t, { deliverAs: "aside" }); return; }
				}
			});
		} catch (err: any) {
			log({ ev: "_on_failed", name, err: String(err?.message ?? err) });
		}
	}
}
