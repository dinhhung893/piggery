// What the plugin shows: piggery top's list, sidebar and events, read from `piggery ps --json` and
// `piggery tail`. The grouping (projects, teams and solos in display order, members as the
// reports_to tree) comes from piggery itself; nothing here groups or sorts. No imports, so the app,
// the daemon and `node --test` all load it as is.

/** The ps --json protocol_version this plugin was written against; another one is shown, not refused. */
export const PROTOCOL_VERSION = 1;

/** A member's current task: the latest mail marked `op: assign` to it, and how its reply chain stands. */
export interface Assignment {
  seq: number;
  title: string;
  from: string;
  at: number;
  /** The newest mail of the chain when it is not the assignment itself: by the member = handed back, else a rework. */
  latest?: { seq: number; at: number; by_member: boolean } | null;
  /** The assigner's newer mail outside the chain (a note, or a task sent without the op). */
  newer?: { seq: number; title: string; at: number } | null;
}

/** A session's own transcript, as its adapter reported it; `piggery tail` reads it. */
export interface Transcript {
  path: string;
  format: string;
}

export interface Member {
  id: string;
  name: string;
  role?: string;
  state: string;
  headless?: boolean;
  gate?: boolean;
  harness?: string;
  model?: string;
  thinking?: string;
  unacked?: number;
  created_at?: number;
  state_since?: number;
  last_turn_end?: number;
  last_activity?: number;
  reports_to?: string;
  spawned_by?: string;
  cwd?: string;
  /** What its harness supports; "usage" means a driver log with ctx, turns and a tail. null: not declared. */
  capabilities?: string[] | null;
  transcript?: Transcript | null;
  assignment?: Assignment | null;
}

export interface Team {
  id: string;
  name: string;
  root?: string;
  gate?: string;
  held?: number;
  unacked?: number;
  created_at?: number;
  members: Member[];
}

export type Solo = Omit<Member, "role" | "headless" | "gate" | "reports_to" | "spawned_by" | "last_turn_end" | "thinking" | "assignment" | "capabilities">;

/** A team top lists as closed: the daemon's closed[] entry. */
export interface ClosedTeam extends Team {
  closed_at?: number;
  closed_by?: string;
}

export interface Unit {
  /** "closed" is a team closed within the last hour, listed like top's All tab. */
  kind: "team" | "solo" | "closed";
  id: string;
  cwd?: string;
  /** A solo's ctx (tokens) and turns, from its transcript, when top shows them. */
  ctx?: number;
  turns?: number;
  /** The members' tree; ctx (tokens) and turns only for a member top shows them for. */
  members?: { id: string; depth: number; prefix: string; cwd: string; ctx?: number; turns?: number }[];
}

export interface Project {
  label: string;
  path: string;
  units: Unit[];
}

export interface Event {
  seq: number;
  ts: number;
  type: string;
  participant?: string | null;
  ref_id?: string | null;
  team_id?: string | null;
}

export interface Ps {
  protocol_version?: number;
  /** The daemon's start (unix ms), build version, and the mail counts top's header shows. */
  started_at?: number;
  version?: string;
  held?: number;
  unacked?: number;
  teams?: Team[] | null;
  closed?: ClosedTeam[] | null;
  solos?: Solo[] | null;
  projects?: Project[] | null;
  events?: Event[] | null;
  names?: Record<string, string> | null;
}

export type Status = "working" | "idle" | "waiting" | "gone";

/** A state's look and word, as piggery top shows them: the word it says and the kind of icon. */
export function stateLook(state: string): { status: Status; word: string } {
  switch (state) {
    case "working":
      return { status: "working", word: "working" };
    case "idle":
      return { status: "idle", word: "idle" };
    case "awaiting_permission":
      return { status: "waiting", word: "waiting" };
    case "gone":
      return { status: "gone", word: "gone" };
    case "requested": // the state is `requested`; the word is what it is for a person, as top says it
      return { status: "waiting", word: "queued" };
  }
  return { status: "waiting", word: state };
}

/** One row of top's list: a team member or a solo session, with every fact top's sidebar has. */
export interface Row {
  id: string;
  name: string;
  /** Tree depth under its team (0 for the gate or a solo). */
  depth: number;
  /** The role; "solo" for a session outside any team. */
  role: string;
  state: string;
  /** "pi", or "pi·worker" for a headless worker, as top. */
  harness: string;
  /** The harness as piggery names it ("pi", "claude", "codex", "cli"), "" when unknown: for its mark. */
  harnessId: string;
  model: string;
  /** The full model and thinking level, as top's Overview ("zai/glm-5.3 · high"). */
  modelFull: string;
  /** Context tokens ("12.3k") and turns over every run; "" where top shows "-". */
  ctx: string;
  turns: string;
  unacked: number;
  created?: number;
  since?: number;
  lastTurn?: number;
  /** Relative to the project, "" in the project directory itself. */
  cwd: string;
  /** The team's gate: its name carries a muted `gate` tag, the team line does not name it. */
  gate: boolean;
  /** Dim like top: gone, or under a gone lead. */
  dim: boolean;
  /** A gone member with nobody live below it: the list folds it into its team's one gone line. */
  folded?: boolean;
  /** Its team's name, null for a solo. */
  team: string | null;
  /** "pi headless worker · gate", "pi session", "solo". */
  kind: string;
  /** "spawned 14:02 by boss", "joined 09:10". */
  came: string;
  reports: string;
  /** Where it runs: the team's root, or the solo's directory. */
  root: string;
  /** Its harness keeps a driver log with ctx, turns and a tail (top's `logged`). */
  logged: boolean;
  /** Piggery has a log of it to tail, as top's `tailable`: a driver log, or the session's transcript. */
  tailable: boolean;
  /** Piggery read ctx and turns for it (ps --json gave a turns count), even when ctx is not known yet. */
  hasStats: boolean;
  /** Its current task; null for a solo, or a member with none. */
  task: Assignment | null;
}

export interface UnitView {
  kind: "team" | "solo" | "closed";
  id: string;
  /** The team's name; null for a solo, whose one row carries its name. */
  title: string | null;
  /** The team's gate (its name), "" when it has none. */
  gate: string;
  held: number;
  /** Whether the unit starts unfolded: a live team; a closed or dead one (below) starts folded. */
  open: boolean;
  /** An open team whose members are all gone, and when it was last active, as top's one line. */
  dead?: { lastActive: number; members: number };
  /** A closed team's when and by whom (an empty `closedBy` is the admin). */
  closedAt?: number;
  closedBy?: string;
  /** A live team's members by state, as top's collapsed line says them: `1 working · 2 idle · 6 gone`, and `unacked N`. */
  counts?: string;
  /** A live team's folded gone members (their rows have `folded`): names in tree order, and when the last went. */
  goneLine?: { names: string[]; last: number };
  rows: Row[];
}

export interface ProjectView {
  /** The directory as ps shortens it, without the leading "…/". */
  title: string;
  path: string;
  units: UnitView[];
}

/** Whether any row of the snapshot (folded, gone and closed ones too) has a directory to show: the Cwd column exists only then. */
export function hasCwd(ps: Ps): boolean {
  return (ps.projects ?? []).some((p) => p.units.some((u) => !!u.cwd || (u.members ?? []).some((m) => !!m.cwd)));
}

/** A team's members by state, then its unacked mail, as top's collapsed team line (parts with a zero are left out). */
export function countsLine(team: Team): string {
  const n = (state: (s: string) => boolean) => team.members.filter((m) => state(m.state)).length;
  const parts = [
    [n((s) => s === "working"), "working"],
    [n((s) => s === "idle"), "idle"],
    [n((s) => s !== "working" && s !== "idle" && s !== "gone"), "waiting"],
    [n((s) => s === "gone"), "gone"],
  ]
    .filter(([count]) => count !== 0)
    .map(([count, what]) => `${count} ${what}`);
  if (parts.length === 0) parts.push("no members");
  if ((team.unacked ?? 0) > 0) parts.push(`unacked ${team.unacked}`);
  return parts.join(" · ");
}

/** An open team with members, every one of them gone: top lists it as one folded line. */
export function isDead(team: Team): boolean {
  return team.members.length > 0 && team.members.every((m) => m.state === "gone");
}

/** A team's latest activity as top counts it: its creation, any member's activity or state change. */
function lastActive(team: Team): number {
  return Math.max(team.created_at ?? 0, ...team.members.map((m) => Math.max(m.last_activity ?? 0, m.state_since ?? 0)));
}

/** A model as ps shows it: the id without its provider ("zai/glm-5.3" is "glm-5.3"). */
export function modelId(model: string | undefined): string {
  if (!model) return "";
  const slash = model.indexOf("/");
  return slash < 0 ? model : model.slice(slash + 1);
}

/** As top's `logged`: the harness declares usage; a member that declared none is judged by being headless. */
function logged(m: Member): boolean {
  return m.capabilities ? m.capabilities.includes("usage") : m.headless === true;
}

function harnessLabel(harness: string | undefined, headless: boolean | undefined): string {
  if (!harness) return "";
  return headless ? `${harness}·worker` : harness;
}

/** Tokens as top shows them: 950, 12.3k, 1.2M. */
export function tokens(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}k`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

function modelLabel(model: string | undefined, thinking: string | undefined): string {
  if (model && thinking) return `${model} · ${thinking}`;
  if (thinking) return `thinking ${thinking}`;
  return model ?? "";
}

function clock(ms: number | undefined): string {
  if (!ms) return "";
  const d = new Date(ms);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

/**
 * The projects in ps order, each unit with its rows. `dir` keeps only the projects that are that
 * directory, inside it or around it (the Farm panel of one workspace). A unit piggery lists but whose
 * team or solo is missing from the same snapshot is skipped.
 */
export function farm(ps: Ps, dir?: string): ProjectView[] {
  const teams = new Map((ps.teams ?? []).map((t) => [t.id, t]));
  const closed = new Map((ps.closed ?? []).map((t) => [t.id, t]));
  const solos = new Map((ps.solos ?? []).map((s) => [s.id, s]));
  const name = names(ps);
  const out: ProjectView[] = [];
  for (const project of ps.projects ?? []) {
    if (dir !== undefined && !related(project.path, dir)) continue;
    const units: UnitView[] = [];
    for (const unit of project.units) {
      if (unit.kind === "solo") {
        const s = solos.get(unit.id);
        if (!s) continue;
        const row: Row = {
          id: s.id,
          name: s.name,
          depth: 0,
          role: "solo",
          state: s.state,
          harness: harnessLabel(s.harness, false),
          harnessId: s.harness ?? "",
          model: modelId(s.model),
          modelFull: modelLabel(s.model, undefined),
          ctx: unit.ctx !== undefined ? tokens(unit.ctx) : "",
          turns: unit.turns !== undefined ? String(unit.turns) : "",
          unacked: s.unacked ?? 0,
          created: s.created_at,
          since: s.state_since,
          cwd: unit.cwd ?? "",
          gate: false,
          dim: false,
          team: null,
          kind: "solo",
          came: s.created_at ? `joined ${clock(s.created_at)}` : "",
          reports: "",
          root: s.cwd ?? project.path,
          logged: false,
          tailable: !!s.transcript?.path,
          hasStats: unit.turns !== undefined,
          task: null,
        };
        units.push({ kind: "solo", id: s.id, title: null, gate: "", held: 0, open: false, rows: [row] });
        continue;
      }
      const closedTeam = unit.kind === "closed" ? closed.get(unit.id) : undefined;
      const team = closedTeam ?? teams.get(unit.id);
      if (!team) continue;
      const members = new Map(team.members.map((m) => [m.id, m]));
      const gone = new Set(team.members.filter((m) => m.state === "gone").map((m) => m.id));
      // Gone members with no live descendant fold into one line, as top's; a gone lead above a live worker keeps its place.
      const liveBelow = new Set<string>();
      for (const m of team.members) {
        if (m.state === "gone") continue;
        for (let up = m.reports_to, seen = 0; up && seen <= team.members.length; up = members.get(up)?.reports_to, seen++) liveBelow.add(up);
      }
      const foldable = closedTeam || isDead(team) ? new Set<string>() : new Set(team.members.filter((m) => m.state === "gone" && !liveBelow.has(m.id)).map((m) => m.id));
      const rows = (unit.members ?? []).flatMap((place): Row[] => {
        const m = members.get(place.id);
        if (!m) return [];
        const kind = [m.harness ? `${m.harness} ${m.headless ? "headless worker" : "session"}` : m.headless ? "headless worker" : "session", m.gate ? "gate" : ""].filter(Boolean).join(" · ");
        const came = m.headless && m.spawned_by ? `spawned ${clock(m.created_at)} by ${name.get(m.spawned_by) ?? "-"}` : `joined ${clock(m.created_at)}`;
        return [
          {
            id: m.id,
            name: m.name,
            depth: place.depth,
            role: m.role ?? "",
            state: m.state,
            harness: harnessLabel(m.harness, m.headless),
            harnessId: m.harness ?? "",
            model: modelId(m.model),
            modelFull: modelLabel(m.model, m.thinking),
            ctx: place.ctx !== undefined ? tokens(place.ctx) : "",
            turns: place.turns !== undefined ? String(place.turns) : "",
            unacked: m.unacked ?? 0,
            created: m.created_at,
            since: m.state_since,
            lastTurn: m.last_turn_end || undefined,
            cwd: place.cwd,
            gate: m.gate === true,
            dim: closedTeam !== undefined || m.state === "gone" || (m.reports_to !== undefined && gone.has(m.reports_to)),
            folded: foldable.has(m.id) || undefined,
            team: team.name,
            kind,
            came,
            reports: m.reports_to ? (name.get(m.reports_to) ?? "") : "",
            root: team.root ?? project.path,
            logged: logged(m),
            tailable: logged(m) || !!m.transcript?.path,
            hasStats: place.turns !== undefined,
            task: m.assignment ?? null,
          },
        ];
      });
      const gate = team.gate ? (name.get(team.gate) ?? team.gate) : "";
      const folded = rows.filter((r) => r.folded);
      const goneLine = folded.length > 0 ? { names: folded.map((r) => r.name), last: Math.max(...team.members.filter((m) => foldable.has(m.id)).map((m) => m.state_since ?? 0)) } : undefined;
      units.push(
        closedTeam
          ? { kind: "closed", id: team.id, title: team.name, gate, held: 0, open: false, closedAt: closedTeam.closed_at, closedBy: closedTeam.closed_by ?? "", rows }
          : isDead(team)
            ? { kind: "team", id: team.id, title: team.name, gate, held: team.held ?? 0, open: false, dead: { lastActive: lastActive(team), members: team.members.length }, rows }
            : { kind: "team", id: team.id, title: team.name, gate, held: team.held ?? 0, open: true, counts: countsLine(team), goneLine, rows },
      );
    }
    out.push({ title: project.label.replace(/^…\//, ""), path: project.path, units });
  }
  return out;
}

function related(path: string, dir: string): boolean {
  const a = path.replace(/\/+$/, "");
  const b = dir.replace(/\/+$/, "");
  return a === b || b.startsWith(a + "/") || a.startsWith(b + "/");
}

/** Names piggery knows for participants and teams. */
function names(ps: Ps): Map<string, string> {
  const out = new Map<string, string>(Object.entries(ps.names ?? {}));
  for (const team of ps.teams ?? []) {
    out.set(team.id, team.name);
    for (const m of team.members) out.set(m.id, m.name);
  }
  for (const team of ps.closed ?? []) {
    out.set(team.id, team.name);
    for (const m of team.members) if (!out.has(m.id)) out.set(m.id, m.name);
  }
  for (const s of ps.solos ?? []) out.set(s.id, s.name);
  return out;
}

/** top's header: the daemon's age and version, team count, working and idle members and sessions, mail counts. */
export interface Summary {
  startedAt?: number;
  version: string;
  teams: number;
  working: number;
  idle: number;
  held: number;
  unacked: number;
}

export function summary(ps: Ps): Summary {
  let working = 0;
  let idle = 0;
  const count = (state: string) => {
    if (state === "working") working++;
    else if (state === "idle") idle++;
  };
  for (const t of ps.teams ?? []) for (const m of t.members) count(m.state);
  for (const s of ps.solos ?? []) count(s.state);
  return { startedAt: ps.started_at, version: ps.version ?? "", teams: (ps.teams ?? []).length, working, idle, held: ps.held ?? 0, unacked: ps.unacked ?? 0 };
}

export type Tone = "muted" | "warning" | "danger";

/**
 * The latest events, newest first, as piggery top lists them: when, who, the event, and its target
 * when that is someone else; denied and held in warning, exited and gone in danger.
 */
export function latestEvents(ps: Ps, count: number): { seq: number; ts: number; who: string; type: string; target: string; tone: Tone }[] {
  const name = names(ps);
  const short = (id: string | null | undefined) => (id ? (name.get(id) ?? (id.length <= 6 ? id : id.slice(-6))) : "");
  return (ps.events ?? [])
    .slice(-count)
    .reverse()
    .map((e) => ({
      seq: e.seq,
      ts: e.ts,
      who: short(e.participant),
      type: e.type,
      target: e.ref_id && e.ref_id !== e.participant && e.ref_id !== e.team_id ? short(e.ref_id) : "",
      tone: e.type === "denied" || e.type === "held" ? "warning" : e.type === "exited" || e.type === "gone" ? "danger" : "muted",
    }));
}

export type TailKind = "tool" | "result" | "error" | "warning" | "rule" | "user" | "text";

/**
 * `piggery tail` text, one entry per line, kind read from its prefix the way piggery top styles it:
 * "> " a tool call, "< " its result ("< name error: …" a failed one), "! " a warning, "-- " a
 * boundary, "user: " the prompt; anything else is the assistant's text.
 */
export function tailLines(text: string): { kind: TailKind; text: string }[] {
  return text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => {
      if (line.startsWith("> ")) return { kind: "tool", text: "▸ " + line.slice(2) };
      if (line.startsWith("< ")) {
        const rest = line.slice(2);
        const colon = rest.indexOf(": ");
        const head = colon < 0 ? rest : rest.slice(0, colon);
        const body = colon < 0 ? "" : rest.slice(colon + 2);
        if (head.endsWith(" error")) return { kind: "error", text: `✗ ${head.slice(0, -" error".length)}  ${body}` };
        return { kind: "result", text: "✓ " + (colon < 0 ? rest : body) };
      }
      if (line.startsWith("! ")) return { kind: "warning", text: line };
      if (line.startsWith("-- ")) return { kind: "rule", text: line };
      if (line.startsWith("user: ")) return { kind: "user", text: line };
      return { kind: "text", text: line.startsWith("assistant: ") ? line.slice("assistant: ".length) : line };
    });
}
