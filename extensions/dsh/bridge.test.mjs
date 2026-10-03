// dsh's real events (committed captures) through the two pure parts: the standard records tail and
// top read, and the standard adapter events the daemon acks by. Batches and acks are the daemon's.
import assert from "node:assert/strict";
import test from "node:test";
import { Turns } from "../pi/adapter.mjs";
import { Bridge } from "./bridge.mjs";
import { Records } from "./records.mjs";
import { events } from "./replay.mjs";

test("records: what was said, the tools, the context in use, one turn_end per model call", () => {
	const out = [];
	const rec = new Records((r) => out.push(r));
	for (const ev of events("basic")) rec.event(ev);
	assert.deepEqual(
		out.map((r) => [r.type, r.message?.role, r.message?.content?.[0]?.text]),
		[
			["message_end", "user", "Reply with exactly the word PONG and nothing else."], // not the runtime context or skills
			["message_end", "assistant", "PONG"],
			["turn_end", undefined, undefined],
			["agent_end", undefined, undefined],
			["message_end", "user", "[mail] From boss: reply with exactly the word MAIL-ACK and nothing else."],
			["message_end", "assistant", "MAIL-ACK"],
			["turn_end", undefined, undefined],
			["agent_end", undefined, undefined],
		],
	);
	// the context as pi counts it: input, output and cache together (dsh's inputTokens leaves cache reads out)
	assert.deepEqual(out[1].message.usage, { input: 5437, output: 11, cacheRead: 0, cacheWrite: 0, totalTokens: 5448 });

	const tools = [];
	const t = new Records((r) => tools.push(r));
	for (const ev of events("midturn")) t.event(ev);
	const calls = tools.filter((r) => r.type.startsWith("tool_"));
	assert.deepEqual(calls.map((r) => [r.type, r.toolName]), Array(3).fill([["tool_execution_start", "bash"], ["tool_execution_end", "bash"]]).flat());
	assert.equal(calls[0].args.command, "sleep 4");
	assert.equal(calls[1].isError, false);

	// a big tool result is cut: the log stays compact
	const big = [];
	const b = new Records((r) => big.push(r));
	b.event({ type: "tool/result", data: { message: { toolCallId: "c", content: [{ type: "text", text: "x".repeat(50000) }] } } });
	assert.ok(JSON.stringify(big[0]).length < 2200);
});

// Plays events into a Bridge over a fake daemon. stops(ev, i, all) says where dsh runs
// agent/turn-stopping: after a step/end with nothing more to do.
async function play(scenario, { stops, answer = () => ({}) }) {
	const calls = [];
	const shown = [];
	let n = 0;
	const turns = new Turns({
		event: async (a) => {
			calls.push(a);
			return answer(a, calls);
		},
		presence: async (e) => calls.push({ presence: e }),
		deliver: (text, steer) => shown.push({ text, steer }),
		newKey: () => `k${++n}`,
		onError: (err) => {
			throw err;
		},
	});
	let dropped = 0;
	const bridge = new Bridge(turns, () => dropped++);
	const all = events(scenario);
	for (const [i, ev] of all.entries()) {
		bridge.event(ev);
		if (stops(ev, i, all)) await bridge.turnStopping();
	}
	await bridge.drain();
	return { calls, shown, dropped };
}

const naturalStop = (ev, i, all) => ev.type === "step/end" && all.slice(i + 1).find((e) => e.type === "turn/end" || e.type === "step/start")?.type === "turn/end";

test("a turn that ends by itself is acked at turn-stopping; two turns, two keys", async () => {
	const { calls, dropped } = await play("basic", { stops: naturalStop });
	assert.deepEqual(calls, [
		{ event: "turn_start", prompt_id: "k1" },
		{ event: "tool_boundary", prompt_id: "k1" },
		{ event: "turn_end", prompt_id: "k1", outcome: "ok" },
		{ presence: "agent_settled" },
		{ event: "turn_start", prompt_id: "k2" },
		{ event: "tool_boundary", prompt_id: "k2" },
		{ event: "turn_end", prompt_id: "k2", outcome: "ok" },
		{ presence: "agent_settled" },
	]);
	assert.equal(dropped, 0);
});

test("mail the daemon hands back at turn-stopping is steered in and the same turn runs on", async () => {
	let first = true;
	const { calls, shown } = await play("stopping", {
		// step 1 ends with nothing more to do, as does step 2; the first stop has mail
		stops: (ev, i, all) => ev.type === "step/end" && (naturalStop(ev, i, all) || ev.data.step === 1),
		answer: (a) => (a.event === "turn_end" && first ? ((first = false), { block: true, text: "MAIL" }) : {}),
	});
	assert.deepEqual(shown, [{ text: "MAIL", steer: true }]);
	assert.deepEqual(
		calls.map((c) => c.event ?? c.presence),
		["turn_start", "tool_boundary", "turn_end", "tool_boundary", "turn_end", "agent_settled"],
	);
	assert.deepEqual(calls.filter((c) => c.event === "turn_end").map((c) => [c.prompt_id, c.outcome]), [["k1", "ok"], ["k1", "ok"]]); // one turn: the daemon acks the last
});

test("an aborted turn ends interrupted (nothing acked, no wake) and its unread mail is dropped, not kept", async () => {
	const { calls, dropped } = await play("abort", { stops: () => false });
	assert.deepEqual(calls.map((c) => c.event ?? c.presence), ["turn_start", "tool_boundary", "turn_end", "agent_settled"]);
	assert.equal(calls[2].outcome, "interrupted");
	assert.equal(dropped, 1);
});

test("a failed turn is a failed end", async () => {
	const all = events("basic");
	const turns = [];
	const t = new Turns({ event: async (a) => turns.push(a), presence: async () => {}, deliver: () => {}, newKey: () => "k", onError: (e) => { throw e; } });
	const bridge = new Bridge(t, () => {});
	for (const ev of all.slice(0, all.findIndex((e) => e.type === "turn/end"))) bridge.event(ev);
	bridge.event({ type: "turn/end", data: { turn: 1, reason: { kind: "error", error: { message: "boom" } } } });
	await bridge.drain();
	assert.deepEqual(turns.at(-1), { event: "turn_end", prompt_id: "k", outcome: "failed" });
	assert.equal(turns.filter((c) => c.event === "turn_end").length, 1); // settle does not end it a second time
});
