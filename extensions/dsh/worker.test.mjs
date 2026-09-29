// The plugin as piggery's worker (`dsh --profile sdk`): its own process, since the plugin keeps
// per-process state and reads what the driver put in the environment when it loads.
import assert from "node:assert/strict";
import test from "node:test";
import { events } from "./replay.mjs";
import { fakeAgent, fakeCtx, fakeDaemon, tempHome, until } from "./fake.mjs";

test("a worker: creates its agent under piggery's session id, is that participant, takes abort and set_model", async (t) => {
	const home = tempHome();
	for (const k of Object.keys(process.env)) if (k.startsWith("PIGGERY_")) delete process.env[k];
	Object.assign(process.env, { PIGGERY_ID: "w1", PIGGERY_TOKEN: "tw", PIGGERY_RUN_ID: "rw", PIGGERY_DSH_SESSION: "sess-w", PIGGERY_DSH_MODEL: "hp/glm-5.3-flash", PIGGERY_DSH_THINKING: "high", PIGGERY_DSH_DENY: "subagent,unknown,workflow" });

	// stdout carries the sdk's JSON-RPC frames and this plugin's records and results; the test runner's own lines pass through
	const out = [];
	const write = process.stdout.write.bind(process.stdout);
	process.stdout.write = (chunk, ...rest) => (String(chunk).startsWith('{"jsonrpc"') ? !!out.push(JSON.parse(String(chunk))) : write(chunk, ...rest));
	t.after(() => (process.stdout.write = write));

	const daemon = await fakeDaemon(home, (f) => {
		if (f.verb === "identify") return { team_id: "T", name: "dev", role: "dev", tools: ["send", "inbox"], role_card: "worker card", run_id: f.args.run_id, protocol_version: 1 };
		return {};
	});
	t.after(() => daemon.close());

	const resolved = [];
	const llm = {
		resolveCallConfig: async (c) => {
			if (c.model === "bad") throw new Error("no adapter for bad");
			resolved.push(c);
		},
	};
	const live = [];
	const ctx = fakeCtx({ agents: live, services: { loader: { await: async () => {} }, llm } });
	const created = [];
	let agent;
	ctx.agents.create = async (o) => {
		created.push(o);
		agent = fakeAgent(o.sessionId, { cwd: o.meta.cwd });
		live.push(agent);
		await ctx.emit("agent/created", { agent, source: "startup" });
		return { agent, dispose() {} };
	};
	const { apply } = await import("./index.mjs");
	apply(ctx);
	t.after(() => {
		ctx.dispose();
		process.stdin.destroy();
	});
	await until(() => created.length === 1);
	const as = (verb) => daemon.calls.filter((c) => c.verb === verb);
	const send = (method, params) => process.stdin.emit("data", Buffer.from(JSON.stringify({ jsonrpc: "2.0", method, params }) + "\n"));

	await t.test("the agent is created as piggery said, and the identity is not left for subprocesses", () => {
		assert.deepEqual(created[0], { sessionId: "sess-w", meta: { cwd: process.cwd() }, agentOptions: { provider: "hp", model: "glm-5.3-flash", reasoningEffort: "high" } });
		assert.deepEqual(Object.keys(process.env).filter((k) => k.startsWith("PIGGERY_")), ["PIGGERY_DISABLED"]);
	});

	await t.test("the worker is told when its agent is up, its level was checked first, the tools it must not have are denied", async () => {
		await until(() => out.some((m) => m.method === "piggery/ready"));
		assert.deepEqual(out.find((m) => m.method === "piggery/ready").params, { session: "sess-w" });
		assert.deepEqual(resolved[0], { provider: "hp", model: "glm-5.3-flash", reasoningEffort: "high" });
		assert.deepEqual(agent.ctx.tools.denied, ["subagent", "workflow"]); // "unknown" is a name dsh refused
	});

	await t.test("it identifies as the worker's run, not a new one, with no join.auto", () => {
		assert.equal(as("join.auto").length, 0);
		const id = as("identify")[0];
		assert.deepEqual([id.as, id.args.run_id, id.args.new_run, id.args.harness, id.args.mode, id.args.harness_ref], ["w1", "rw", false, "dsh", "rpc", "sess-w"]);
		assert.deepEqual(Object.keys(agent.tools).sort(), ["piggery_inbox", "piggery_send"]);
		assert.equal(agent.sections["piggery-role"].text(), "worker card");
	});

	await t.test("abort from the driver cancels the turn and keeps the inbox", () => {
		send("piggery/abort", {});
		assert.deepEqual(agent.cancels, [{ cause: { kind: "hook", reason: "piggery abort" }, opts: { keepInbox: true } }]);
	});

	await t.test("set_model is checked with dsh's llm service, answered, and used from the next request; a refusal changes nothing", async () => {
		send("piggery/set_model", { id: "piggery-1", model: "hp/kimi-k3", thinking: "max" });
		await until(() => out.some((m) => m.method === "piggery/result" && m.params.id === "piggery-1"));
		assert.deepEqual(out.find((m) => m.params.id === "piggery-1").params, { id: "piggery-1", ok: true });
		assert.deepEqual(resolved.at(-1), { provider: "hp", model: "kimi-k3", reasoningEffort: "max" });
		const [{ fn }] = ctx.handlers["agent/request"];
		assert.deepEqual(await fn({ agent }, async () => ({ provider: "hp", model: "glm-5.3-flash", reasoningEffort: "high", maxTokens: 9 })), { provider: "hp", model: "kimi-k3", reasoningEffort: "max", maxTokens: 9 });

		send("piggery/set_model", { id: "piggery-2", model: "hp/bad" });
		await until(() => out.some((m) => m.params?.id === "piggery-2"));
		assert.deepEqual(out.find((m) => m.params.id === "piggery-2").params, { id: "piggery-2", ok: false, error: "no adapter for bad" });
		assert.equal((await fn({ agent }, async () => ({ provider: "hp", model: "glm-5.3-flash" }))).model, "kimi-k3");

		send("piggery/set_model", { id: "piggery-3", thinking: "" }); // only the level: cleared, the model stays
		await until(() => out.some((m) => m.params?.id === "piggery-3"));
		assert.deepEqual(await fn({ agent }, async () => ({ provider: "hp", model: "glm-5.3-flash", reasoningEffort: "high" })), { provider: "hp", model: "kimi-k3" });
	});

	await t.test("its records are the stdout the driver logs", async () => {
		for (const ev of events("basic").slice(0, 20)) ctx.session(agent, ev);
		const recs = out.filter((m) => m.method === "piggery/record").map((m) => m.params);
		assert.ok(recs.some((r) => r.type === "message_end" && r.message.role === "assistant" && r.message.content[0].text === "PONG"));
	});
});
