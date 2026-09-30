import assert from "node:assert/strict";
import { chmodSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, realpathSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { piggeryPath } from "../server/installed.ts";
import { outdatedNotice, pickBin, realDir, runPiggery } from "../server/piggery.ts";
import { foldsOf, goneOpen, NO_FOLDS, prune, setEvents, setGone, setTeam, teamOpen } from "../shared/folds.ts";
import { RPC_NAMES, viewSettings } from "../shared/rpc.ts";
import { apart, columnsFor, EVENT_SLOTS, slotWidth } from "../client/kit/theme.ts";
import { clock } from "../client/poll.ts";
import { countsLine, farm, hasCwd, latestEvents, stateLook, summary, tailLines, tokens, type Ps } from "../shared/view.ts";

// A real `piggery ps --json`, paths made neutral.
const ps = JSON.parse(readFileSync(new URL("./fixtures/ps.json", import.meta.url), "utf8")) as Ps;

test("farm keeps piggery's grouping and order, with top's facts per row", () => {
  const view = farm(ps);
  assert.deepEqual(
    view.map((p) => [p.title, p.units.map((u) => [u.kind, u.title ?? u.rows[0].name])]),
    [
      ["labs/shop", [["solo", "vital-plain"], ["solo", "icy-oasis"]]],
      ["labs/api", [["team", "api"]]],
    ],
  );
  const team = view[1].units[0];
  assert.deepEqual([team.gate, team.held], ["", 0]);
  assert.deepEqual(
    team.rows.map((r) => [r.name, r.depth, r.role, r.harness, r.model, r.dim, r.logged]),
    [
      ["pi-1366de", 0, "planner", "pi", "glm-5.3", true, false],
      ["dev-1", 1, "dev", "pi·worker", "glm-5.3", true, true],
      ["dev-2", 1, "dev", "pi·worker", "glm-5.3", true, true],
    ],
  );
  assert.equal(team.rows[1].kind, "pi headless worker");
  assert.match(team.rows[1].came, /^spawned \d\d:\d\d by pi-1366de$/);
  const solo = view[0].units[0].rows[0];
  assert.deepEqual([solo.role, solo.kind, solo.root, solo.logged], ["solo", "solo", "/home/dev/src/labs/shop", false]);
});

test("a team whose members are all gone starts folded, and unfolded again when one is back", () => {
  const dead = farm(ps)[1].units[0];
  assert.deepEqual([dead.open, dead.dead?.members], [false, 3]);
  const [lead, ...rest] = ps.teams![0].members;
  const back: Ps = { ...ps, teams: [{ ...ps.teams![0], members: [{ ...lead, state: "idle" }, ...rest] }] };
  const live = farm(back)[1].units[0];
  assert.deepEqual([live.open, live.dead], [true, undefined]);
});

test("in a live team the gone members with nobody live below them fold into one line; a gone lead above a live worker stays", () => {
  const [lead, w1, w2] = ps.teams![0].members;
  const live: Ps = { ...ps, teams: [{ ...ps.teams![0], members: [lead, { ...w1, state: "idle" }, w2] }] };
  const team = farm(live)[1].units[0];
  assert.deepEqual(team.rows.map((r) => [r.name, !!r.folded]), [["pi-1366de", false], ["dev-1", false], ["dev-2", true]]);
  assert.deepEqual([team.goneLine?.names, team.goneLine?.last], [["dev-2"], w2.state_since]);
  assert.equal(farm(ps)[1].units[0].goneLine, undefined, "a dead team is one folded line already");
});

test("farm for a workspace keeps the projects at, inside or around its directory", () => {
  assert.deepEqual(farm(ps, "/home/dev/src/labs/api").map((p) => p.title), ["labs/api"]);
  assert.deepEqual(farm(ps, "/home/dev/src/labs/api/web").map((p) => p.title), ["labs/api"]);
  assert.deepEqual(farm(ps, "/home/dev/src/labs").map((p) => p.title), ["labs/shop", "labs/api"]);
  assert.deepEqual(farm(ps, "/home/dev/src/labs/ap"), []);
});

test("ctx, turns, thinking and recently closed teams come from ps --json as top reads them", () => {
  const team = ps.teams![0];
  const [lead, w1] = team.members;
  const project = ps.projects![1];
  const withTop: Ps = {
    ...ps,
    teams: [{ ...team, members: team.members.map((m) => (m.id === w1.id ? { ...m, thinking: "high" } : m)) }],
    closed: [{ id: "T-old", name: "old", members: [{ ...lead, id: "M-old", name: "gone-lead" }], closed_at: 1, closed_by: "boss" }],
    projects: [
      ps.projects![0],
      {
        ...project,
        units: [
          { ...project.units[0], members: project.units[0].members!.map((m) => (m.id === w1.id ? { ...m, ctx: 12345, turns: 7 } : m)) },
          { kind: "closed", id: "T-old", members: [{ id: "M-old", depth: 0, prefix: "", cwd: "" }] },
        ],
      },
    ],
  };
  const [open, closed] = farm(withTop)[1].units;
  const worker = open.rows.find((r) => r.id === w1.id)!;
  assert.deepEqual([worker.ctx, worker.turns, worker.modelFull], ["12.3k", "7", "zai/glm-5.3 · high"]);
  assert.deepEqual([open.rows[0].ctx, open.rows[0].turns], ["", ""]);
  assert.deepEqual([closed.kind, closed.title, closed.closedBy, closed.rows.map((r) => [r.name, r.dim])], ["closed", "old", "boss", [["gone-lead", true]]]);
  assert.deepEqual([tokens(950), tokens(12345), tokens(2_500_000)], ["950", "12.3k", "2.5M"]);
});

test("a session with a transcript has a tail and ctx/turns as top does; a member's task and the header come from ps --json", () => {
  const team = ps.teams![0];
  const [lead, w1] = team.members;
  const solo = ps.solos![0];
  const task = { seq: 175, title: "fix the store", from: "pi-1366de", at: 5, latest: { seq: 180, at: 6, by_member: true }, newer: { seq: 182, title: "a note", at: 7 } };
  const withSessions: Ps = {
    ...ps,
    started_at: 1,
    version: "dev-abc",
    held: 2,
    unacked: 3,
    solos: [{ ...solo, transcript: { path: "/s/a.jsonl", format: "claude" } }, ...ps.solos!.slice(1)],
    teams: [
      {
        ...team,
        members: [
          { ...lead, transcript: { path: "/s/lead.jsonl", format: "pi" }, capabilities: ["wake", "steer"] },
          { ...w1, capabilities: ["usage"], assignment: task },
          ...team.members.slice(2),
        ],
      },
    ],
    projects: [
      { ...ps.projects![0], units: [{ ...ps.projects![0].units[0], ctx: 900, turns: 4 }, ps.projects![0].units[1]] },
      {
        ...ps.projects![1],
        units: [{ ...ps.projects![1].units[0], members: ps.projects![1].units[0].members!.map((m) => (m.id === lead.id ? { ...m, ctx: 12345, turns: 9 } : m)) }],
      },
    ],
  };
  const [shop, api] = farm(withSessions);
  const [s1, s2] = shop.units.map((u) => u.rows[0]);
  assert.deepEqual([s1.tailable, s1.hasStats, s1.ctx, s1.turns], [true, true, "900", "4"]);
  assert.deepEqual([s2.tailable, s2.hasStats], [false, false], "no transcript, no log: a clear empty state");
  const [l, w, w2] = api.units[0].rows;
  assert.deepEqual([l.logged, l.tailable, l.hasStats, l.ctx, l.task], [false, true, true, "12.3k", null]);
  assert.deepEqual([w.logged, w.tailable, w.task], [true, true, task]);
  assert.deepEqual([w2.logged, w2.tailable, w2.hasStats], [true, true, false], "a worker that declared no capabilities is judged by being headless");
  assert.deepEqual(summary(withSessions), { startedAt: 1, version: "dev-abc", teams: 1, working: 0, idle: 2, held: 2, unacked: 3 }, "the two solos are idle; the team is all gone");
});

test("events are top's: newest first, who and target by name, denied/held and exited/gone coloured", () => {
  const [lead, w1] = ps.teams![0].members;
  const events = latestEvents(
    {
      ...ps,
      events: [
        { seq: 1, ts: 1, type: "spawned", participant: lead.id, ref_id: w1.id },
        { seq: 2, ts: 2, type: "held", participant: w1.id, ref_id: "01MSGIDXYZ123456" },
        { seq: 3, ts: 3, type: "exited", participant: w1.id },
      ],
    },
    8,
  );
  assert.deepEqual(
    events.map((e) => [e.who, e.type, e.target, e.tone]),
    [
      ["dev-1", "exited", "", "danger"],
      ["dev-1", "held", "123456", "warning"],
      ["pi-1366de", "spawned", "dev-1", "muted"],
    ],
  );
  assert.equal(latestEvents(ps, 8).length, 8);
});

test("tail lines take their kind from piggery tail's prefixes", () => {
  const text = [
    "-- agent end",
    "user: [piggery] 1 new message",
    "> bash ls -la",
    "< bash: 3 files",
    "< bash error: exit 1",
    "! retrying",
    "assistant: Done.",
    "",
  ].join("\n");
  assert.deepEqual(tailLines(text), [
    { kind: "rule", text: "-- agent end" },
    { kind: "user", text: "user: [piggery] 1 new message" },
    { kind: "tool", text: "▸ bash ls -la" },
    { kind: "result", text: "✓ 3 files" },
    { kind: "error", text: "✗ bash  exit 1" },
    { kind: "warning", text: "! retrying" },
    { kind: "text", text: "Done." },
  ]);
});

test("RPC names are ones Paseo accepts (one bad name fails the whole plugin)", () => {
  for (const name of RPC_NAMES) assert.match(name, /^[a-z][a-z0-9._-]*$/);
});

test("runPiggery always passes --no-start and says why piggery gave nothing", async () => {
  const dir = mkdtempSync(join(tmpdir(), "piggery-paseo-"));
  const fake = (name: string, body: string) => {
    const path = join(dir, name);
    writeFileSync(path, `#!/bin/sh\n${body}\n`);
    chmodSync(path, 0o755);
    return path;
  };
  const echo = await runPiggery(fake("echo", 'echo "$@"'), ["ps", "--json"]);
  assert.deepEqual(echo, { ok: true, out: "ps --json --no-start\n" });

  const down = await runPiggery(fake("down", 'echo "the piggery daemon is not running" >&2; exit 1'), ["ps"]);
  assert.equal(down.ok || down.code, "down");

  const old = await runPiggery(fake("old", 'echo "ps: flag provided but not defined: -no-start" >&2; exit 2'), ["ps"]);
  assert.match(old.ok ? "" : old.error, /older than this plugin/);

  const missing = await runPiggery(join(dir, "nope"), ["ps"]);
  assert.equal(missing.ok || missing.code, "missing");

  const slow = await runPiggery(fake("slow", "sleep 5"), ["ps"], 200);
  assert.equal(slow.ok || slow.code, "failed");
  assert.match(slow.ok ? "" : slow.error, /did not answer/);
});

test("the outdated notice is built from ps --json's list, and spawns nothing", () => {
  assert.equal(outdatedNotice({}), "", "an older piggery reports no list");
  assert.equal(outdatedNotice({ outdated: [] }), "");
  assert.equal(
    outdatedNotice({ outdated: [{ name: "codex", have: 0, want: 1 }, { name: "claude", have: 1, want: 1, drift: "6 of 7 hooks" }, { name: 3 }] }),
    "outdated: codex (v0 < v1), claude (v1: 6 of 7 hooks): piggery setup --outdated",
  );
});

test("the binary is the user's setting, else the one setup installed, else piggery on PATH", () => {
  assert.equal(piggeryPath, "", "the source tree ships no machine path; setup writes it");
  assert.equal(pickBin(" /opt/piggery ", "/usr/local/bin/piggery"), "/opt/piggery");
  assert.equal(pickBin("", "/usr/local/bin/piggery"), "/usr/local/bin/piggery");
  assert.equal(pickBin("", ""), "piggery");
});

test("components take text sizes and weights from the text roles only", () => {
  const dir = new URL("../client/", import.meta.url);
  const files = readdirSync(dir, { recursive: true }).map(String).filter((f) => f.endsWith(".tsx"));
  assert.ok(files.some((f) => f.startsWith("kit")), "the kit's components are checked too");
  for (const file of files) {
    assert.doesNotMatch(readFileSync(new URL(file, dir), "utf8"), /fontSize|fontWeight/, file);
  }
});

test("a workspace reached through a symlink matches the directory piggery recorded", async () => {
  const base = realpathSync(mkdtempSync(join(tmpdir(), "piggery-paseo-")));
  mkdirSync(join(base, "shop"));
  symlinkSync(join(base, "shop"), join(base, "link"));
  const recorded: Ps = { projects: [{ label: "…/shop", path: join(base, "shop"), units: [] }] };
  assert.deepEqual(farm(recorded, join(base, "link")), [], "the workspace path as given does not match");
  assert.deepEqual(farm(recorded, await realDir(join(base, "link"))).map((p) => p.path), [join(base, "shop")]);
  assert.equal(await realDir(join(base, "nowhere")), join(base, "nowhere"));
});

test("an event's time is the local clock: seconds today, month-day and minutes on an earlier day", () => {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 9, 4, 5);
  const earlier = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 0);
  assert.equal(clock(today.getTime()), "09:04:05");
  assert.equal(clock(earlier.getTime()), `${String(earlier.getMonth() + 1).padStart(2, "0")}-${String(earlier.getDate()).padStart(2, "0")} 23:59`);
  assert.equal(clock(undefined), "");
});

test("a folded live team says its members by state and its unacked mail, as top's collapsed line", () => {
  const [lead, w1, w2] = ps.teams![0].members;
  const team = { ...ps.teams![0], unacked: 2, members: [{ ...lead, state: "working" }, { ...w1, state: "idle" }, { ...w2, state: "gone" }, { ...w2, id: "x", state: "gone" }] };
  assert.equal(countsLine(team), "1 working · 1 idle · 2 gone · unacked 2");
  assert.equal(countsLine({ ...team, unacked: 0, members: [] }), "no members");
  assert.equal(farm({ ...ps, teams: [team] })[1].units[0].counts, "1 working · 1 idle · 2 gone · unacked 2");
});

test("folds are kept as differences from the defaults, restored from the host's setting, and dropped with their team", () => {
  let f = setTeam(NO_FOLDS, "live", false, true); // a live team folded
  f = setTeam(f, "dead", false, false); // the default again: nothing to keep
  f = setGone(f, "live", true);
  f = setGone(setEvents(f, true), "vanished", true); // Events opened
  f = setTeam(f, "vanished", false, true);
  assert.deepEqual(f.teams, { live: false, vanished: false });
  const saved = prune(f, ["live", "dead"]);
  assert.deepEqual(saved, { teams: { live: false }, gone: { live: true }, eventsOpen: true });
  const back = foldsOf(viewSettings.schema.parse(JSON.parse(JSON.stringify(saved))));
  assert.deepEqual([teamOpen(back, "live", true), teamOpen(back, "dead", false), goneOpen(back, "live"), back.eventsOpen], [false, false, true, true]);
  assert.deepEqual(foldsOf(viewSettings.schema.parse({})), NO_FOLDS, "a new install starts unfolded");
  assert.equal(setEvents(back, false).eventsOpen, false);
  assert.equal(foldsOf(viewSettings.schema.parse({ events: true })).eventsOpen, false, "the old key (events: true meant folded) does not come back as open");
});

test("an event's fields fall under member columns at every list width: the slots cover the shown columns in order", () => {
  for (const width of [1400, 1100, 1050, 1000, 950, 900, 850, 800, 760, 700, 620, 550, 450, 300]) {
    const slots = EVENT_SLOTS.map((s) => slotWidth(width, s.keys));
    assert.equal(slots.reduce((a, b) => a + b, 0), columnsFor(width).reduce((a, c) => a + c.width, 0), `at ${width}`);
    // each slot starts where its first shown column starts
    let at = 0;
    const starts = new Map<string, number>();
    for (const c of columnsFor(width)) {
      starts.set(c.key, at);
      at += c.width;
    }
    let slotAt = 0;
    EVENT_SLOTS.forEach((s, i) => {
      const first = columnsFor(width).find((c) => s.keys.includes(c.key));
      if (first) assert.equal(slotAt, starts.get(first.key), `${s.what} at ${width}`);
      slotAt += slots[i];
    });
  }
  assert.equal(slotWidth(1400, ["age", "since"]), 96, "the time slot is Age and Since");
});

test("a left-aligned column right after a right-aligned one is set apart, at every width", () => {
  for (const width of [1400, 1100, 1050, 1000, 940, 800, 600, 400]) {
    const cols = columnsFor(width);
    const apartKeys = cols.filter((c) => apart(width, c.key)).map((c) => c.key);
    assert.deepEqual(apartKeys, width >= 1100 ? ["cwd"] : [], `at ${width}`);
  }
  assert.equal(apart(1400, "model"), false, "model follows harness, a left column");
});

test("the gate member's row carries the gate flag; no other row does, and a solo never", () => {
  const [lead, ...rest] = ps.teams![0].members;
  const gated: Ps = { ...ps, teams: [{ ...ps.teams![0], members: [{ ...lead, gate: true }, ...rest] }] };
  const rows = farm(gated).flatMap((p) => p.units.flatMap((u) => u.rows));
  assert.deepEqual(rows.filter((r) => r.gate).map((r) => r.name), [lead.name]);
});

test("requested reads as queued, and the Cwd column exists only while some row (a folded gone one too) has a directory", () => {
  assert.equal(stateLook("requested").word, "queued");
  assert.equal(hasCwd(ps), false, "the fixture's rows are all in their directories");
  const unit = ps.projects![1].units[0];
  const withCwd = (id: string): Ps => ({
    ...ps,
    projects: [ps.projects![0], { ...ps.projects![1], units: [{ ...unit, members: unit.members!.map((m) => (m.id === id ? { ...m, cwd: "sub" } : m)) }] }],
  });
  const gone = ps.teams![0].members.find((m) => m.state === "gone")!;
  assert.equal(hasCwd(withCwd(gone.id)), true);
  assert.equal(columnsFor(1400, false).some((c) => c.key === "cwd"), false);
  assert.equal(slotWidth(1400, ["cwd"], false), 0, "event rows' trailing spacer goes with the column");
});
