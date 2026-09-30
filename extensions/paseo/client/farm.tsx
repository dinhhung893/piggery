import type { PluginTheme } from "@getpaseo/plugin";
import { useRpc, useSettings, useWorkspace, type PluginSurfaceProps, type PluginWorkspacePanelProps } from "@getpaseo/plugin/client";
import { Icon, ScrollView } from "@getpaseo/plugin/client/react-native";
import { useEffect, useRef, useState } from "react";
import { Text, View } from "react-native";
import { foldsOf, goneOpen as goneIsOpen, NO_FOLDS, prune, setEvents, setGone, setTeam, teamOpen, type Folds } from "../shared/folds.ts";
import { snapshot, viewSettings } from "../shared/rpc.ts";
import { farm, hasCwd, latestEvents, PROTOCOL_VERSION, stateLook, summary, type Ps, type Row as PigRow, type UnitView } from "../shared/view.ts";
import { Detail } from "./detail.tsx";
import { Disclosure } from "./kit/disclosure.tsx";
import { HarnessMark } from "./kit/harness-mark.tsx";
import { Cell, GroupLabel, ListRow, Rows } from "./kit/list.tsx";
import { StateDot } from "./kit/mark.tsx";
import { apart, CHEVRON_SLOT, columnsFor, EVENT_SLOTS, ICON_SIZE, INDENT, NAME_MIN, slotWidth, SPACING, SPLIT_MIN, numeric, text, type ColumnKey } from "./kit/theme.ts";
import { ago, clock, usePoll } from "./poll.ts";

const EVENTS = 8;

/** ps --json, and with `dir` that directory as piggery writes paths (resolved on the daemon side). */
function useSnapshot(dir: string | undefined) {
  const call = useRpc(snapshot);
  return usePoll(async () => {
    const r = await call({ dir });
    return r.ok ? { ok: true, value: { ps: r.ps as Ps, dir: r.dir ?? dir, outdated: r.outdated ?? "" } } : r;
  }, `snapshot:${dir ?? ""}`);
}

/** The list's width and whether any row has a directory: what decides its columns. */
interface Grid {
  width: number;
  cwd: boolean;
}

/** The columns that fit a list this wide, in top's order. */
function shown(grid: Grid) {
  return columnsFor(grid.width, grid.cwd);
}

function value(row: PigRow, key: Exclude<ColumnKey, "state">): string {
  switch (key) {
    case "role":
      return row.role;
    case "harness":
      return row.harness;
    case "model":
      return row.model;
    case "ctx":
      return row.ctx;
    case "turns":
      return row.turns;
    case "age":
      return ago(row.created);
    case "since":
      return ago(row.since);
    case "cwd":
      return row.cwd && `./${row.cwd}`;
  }
}

/** The trailing slot: a chevron that says the row opens its detail (accent while selected). */
function Opens({ theme, selected }: { theme: PluginTheme; selected?: boolean }) {
  return (
    <View style={{ width: SPACING[6], alignItems: "flex-end" }}>
      {selected !== undefined ? <Icon name="ChevronRight" size={ICON_SIZE.sm} color={selected ? theme.colors.accent : theme.colors.foregroundMuted} /> : null}
    </View>
  );
}

/** The column labels, once above every group (top's header), as the host's small muted labels. */
function Header({ theme, grid }: { theme: PluginTheme; grid: Grid }) {
  return (
    <View style={{ flexDirection: "row", alignItems: "center", paddingHorizontal: SPACING[4], paddingTop: SPACING[3], paddingBottom: SPACING[1.5] }}>
      <Text style={[text(theme, "label"), { flex: 1, minWidth: NAME_MIN }]}>Name</Text>
      {shown(grid).map((c) => (
        <Cell key={c.key} theme={theme} width={c.width} right={numeric(c.key)} apart={apart(grid.width, c.key, grid.cwd)} head>
          {c.label}
        </Cell>
      ))}
      <Opens theme={theme} />
    </View>
  );
}

function MemberRow({ theme, row, grid, indent, selected, onSelect }: { theme: PluginTheme; row: PigRow; grid: Grid; indent: number; selected: boolean; onSelect: () => void }) {
  const look = stateLook(row.state);
  return (
    <ListRow theme={theme} onPress={onSelect} selected={selected}>
      <View style={{ flex: 1, minWidth: NAME_MIN, flexDirection: "row", alignItems: "center", gap: SPACING[2], paddingLeft: indent }}>
        <HarnessMark theme={theme} harness={row.harnessId} muted={row.dim} />
        <Text style={[text(theme, "rowTitle", row.dim ? "foregroundMuted" : "foreground"), { flexShrink: 1 }]} numberOfLines={1}>
          {row.name}
        </Text>
        {row.gate ? <Text style={[text(theme, "meta"), { flexShrink: 0 }]}>gate</Text> : null}
      </View>
      {shown(grid).map((c) =>
        c.key === "state" ? (
          <View key={c.key} style={{ width: c.width, paddingLeft: SPACING[2] }}>
            <StateDot theme={theme} status={look.status} word={look.word} />
          </View>
        ) : (
          <Cell key={c.key} theme={theme} width={c.width} right={numeric(c.key)} apart={apart(grid.width, c.key, grid.cwd)}>
            {value(row, c.key) || "-"}
          </Cell>
        ),
      )}
      <Opens theme={theme} selected={selected} />
    </ListRow>
  );
}

/** A live team: chevron · name, then `no gate` when it has none and held mail as top's title, and, folded, its counts. */
function TeamRow({ theme, unit, open, onToggle }: { theme: PluginTheme; unit: UnitView; open: boolean; onToggle: () => void }) {
  return (
    <Disclosure theme={theme} open={open} onToggle={onToggle}>
      <Icon name="Users" size={ICON_SIZE.md} color={theme.colors.foreground} />
      <Text style={text(theme, "rowTitle")} numberOfLines={1}>
        {unit.title}
      </Text>
      {!unit.gate || unit.held > 0 ? ( // the gate is tagged on its member's row
        <Text style={[text(theme, "meta"), { flexShrink: 1 }]} numberOfLines={1}>
          {unit.gate ? null : <Text style={text(theme, "meta", "statusWarning")}>no gate</Text>}
          {unit.held > 0 ? <Text style={text(theme, "meta", "statusWarning")}>{`${unit.gate ? "" : " · "}${unit.held} held`}</Text> : null}
        </Text>
      ) : null}
      {!open && unit.counts ? (
        // its own text, a gap after the gate: inline spaces collapse, and "gate ocean 1 idle" reads as one phrase
        <Text style={[text(theme, "meta"), { flexShrink: 1, marginLeft: SPACING[2] }]} numberOfLines={1}>
          {unit.counts}
        </Text>
      ) : null}
    </Disclosure>
  );
}

/**
 * A team listed as one line (closed, or open with every member gone), as a row on the member grid like
 * top's: chevron, icon, name and `closed` for a closed one; a gone state; when it was last active or
 * closed under Since. Dim; the other cells empty.
 */
function DeadTeamRow({ theme, unit, grid, open, onToggle }: { theme: PluginTheme; unit: UnitView; grid: Grid; open: boolean; onToggle: () => void }) {
  const closed = unit.kind === "closed";
  const at = closed ? unit.closedAt : unit.dead?.lastActive;
  return (
    <ListRow theme={theme} onPress={onToggle} expanded={open}>
      <View style={{ flex: 1, minWidth: NAME_MIN, flexDirection: "row", alignItems: "center", gap: SPACING[2] }}>
        <Icon name={open ? "ChevronDown" : "ChevronRight"} size={ICON_SIZE.sm} color={theme.colors.foregroundMuted} />
        <Icon name={closed ? "Archive" : "Users"} size={ICON_SIZE.md} color={theme.colors.foregroundMuted} />
        <Text style={[text(theme, "rowTitle", "foregroundMuted"), { flexShrink: 1 }]} numberOfLines={1}>
          {unit.title}
        </Text>
        {closed ? <Text style={text(theme, "meta")}>closed</Text> : null}
      </View>
      {shown(grid).map((c) =>
        c.key === "state" ? (
          <View key={c.key} style={{ width: c.width, paddingLeft: SPACING[2] }}>
            <StateDot theme={theme} status="gone" word="gone" />
          </View>
        ) : c.key === "since" ? (
          <Cell key={c.key} theme={theme} width={c.width} right>
            {ago(at)}
          </Cell>
        ) : (
          <View key={c.key} style={{ width: c.width }} />
        ),
      )}
      <Opens theme={theme} />
    </ListRow>
  );
}

/**
 * A live team's gone members with nobody live below them, as one row on the member grid, like top's:
 * `▸ 6 members` in the name box, a gone state, the latest time one went gone under Since. Dim; the rest empty.
 */
function GoneRow({ theme, line, grid, open, onToggle }: { theme: PluginTheme; line: { names: string[]; last: number }; grid: Grid; open: boolean; onToggle: () => void }) {
  const count = line.names.length;
  return (
    <ListRow theme={theme} onPress={onToggle} expanded={open}>
      <View style={{ flex: 1, minWidth: NAME_MIN, flexDirection: "row", alignItems: "center", gap: SPACING[2], paddingLeft: CHEVRON_SLOT }}>
        <View style={{ width: ICON_SIZE.md, alignItems: "center" }}>
          <Icon name={open ? "ChevronDown" : "ChevronRight"} size={ICON_SIZE.sm} color={theme.colors.foregroundMuted} />
        </View>
        <Text style={[text(theme, "rowTitle", "foregroundMuted"), { flexShrink: 1 }]} numberOfLines={1}>
          {`${count} member${count === 1 ? "" : "s"}`}
        </Text>
      </View>
      {shown(grid).map((c) =>
        c.key === "state" ? (
          <View key={c.key} style={{ width: c.width, paddingLeft: SPACING[2] }}>
            <StateDot theme={theme} status="gone" word="gone" />
          </View>
        ) : c.key === "since" ? (
          <Cell key={c.key} theme={theme} width={c.width} right>
            {ago(line.last)}
          </Cell>
        ) : (
          <View key={c.key} style={{ width: c.width }} />
        ),
      )}
      <Opens theme={theme} />
    </ListRow>
  );
}

/** An event's small mark by its kind, muted; one per row. */
const EVENT_ICON: Record<string, string> = { spawned: "Plus", team_up: "ArrowUp", team_down: "ArrowDown", gc: "Trash2" };

/** An event's time, right-aligned to the right edge of its slot; a slot narrower than the text (Age hidden) lets it run left over the empty target slot. */
function TimeCell({ theme, width, time }: { theme: PluginTheme; width: number; time: string }) {
  const full = slotWidth(Infinity, ["age", "since"]); // both, whatever is shown
  if (width >= full) return <Cell theme={theme} width={width} right>{time}</Cell>;
  return (
    <View style={{ width, flexDirection: "row", justifyContent: "flex-end" }}>
      <View style={{ width: full, flexShrink: 0 }}>
        <Cell theme={theme} width={full} right>
          {time}
        </Cell>
      </View>
    </View>
  );
}

/** The latest events as top lists them: when, who, what, its target. Its label folds them to the latest one, as `e` does in top. */
function Events({ theme, ps, grid, folded, onToggle }: { theme: PluginTheme; ps: Ps; grid: Grid; folded: boolean; onToggle: () => void }) {
  const events = latestEvents(ps, EVENTS);
  if (events.length === 0) return null;
  const colourOf = (tone: string) => (tone === "danger" ? "statusDanger" : tone === "warning" ? "statusWarning" : "foreground");
  const latest = events[0];
  return (
    <>
      <Disclosure theme={theme} open={!folded} onToggle={onToggle}>
        <Icon name="Activity" size={ICON_SIZE.sm} color={theme.colors.foregroundMuted} />
        <Text style={[text(theme, "label"), { flexShrink: 1 }]} numberOfLines={1}>
          Events
          {folded ? <Text style={text(theme, "meta", colourOf(latest.tone))}>{` · ${[clock(latest.ts), latest.who, latest.type, latest.target].filter(Boolean).join(" ")}`}</Text> : null}
        </Text>
      </Disclosure>
      <Rows theme={theme}>
        {folded
          ? []
          : events.map((e) => {
              const colour = colourOf(e.tone);
              return (
                <ListRow key={e.seq} theme={theme}>
                  <View style={{ flex: 1, minWidth: NAME_MIN, flexDirection: "row", alignItems: "center", gap: SPACING[2] }}>
                    <Icon name={EVENT_ICON[e.type] ?? "Dot"} size={ICON_SIZE.md} color={theme.colors.foregroundMuted} />
                    <Text style={[text(theme, "rowTitle"), { flexShrink: 1 }]} numberOfLines={1}>
                      {e.who || "-"}
                    </Text>
                  </View>
                  {EVENT_SLOTS.map((slot) => {
                    const w = slotWidth(grid.width, slot.keys, grid.cwd);
                    if (w === 0) return null;
                    if (slot.what === "type") return <Cell key={slot.what} theme={theme} width={w} colour={colour}>{e.type}</Cell>;
                    if (slot.what === "target") return <Cell key={slot.what} theme={theme} width={w}>{e.target}</Cell>;
                    if (slot.what === "time") return <TimeCell key={slot.what} theme={theme} width={w} time={clock(e.ts)} />;
                    return <View key={slot.what} style={{ width: w }} />;
                  })}
                  <Opens theme={theme} />
                </ListRow>
              );
            })}
      </Rows>
    </>
  );
}

/** top's header line: the daemon's age, teams, working and idle, held (amber when any) and unacked mail, its build version; under it, amber, what needs `piggery setup`. */
function Status({ theme, ps, outdated }: { theme: PluginTheme; ps: Ps; outdated: string }) {
  const s = summary(ps);
  return (
    <Text style={[text(theme, "meta"), { paddingHorizontal: SPACING[4], paddingTop: SPACING[3] }]} numberOfLines={3}>
      {`daemon ${s.startedAt ? `${ago(s.startedAt)} up` : "up"} · ${s.teams} team${s.teams === 1 ? "" : "s"} · ${s.working} working · ${s.idle} idle · `}
      <Text style={text(theme, "meta", s.held > 0 ? "statusWarning" : undefined)}>{`held ${s.held}`}</Text>
      {` · unacked ${s.unacked}${s.version ? ` · ${s.version}` : ""}`}
      {outdated ? <Text style={text(theme, "meta", "statusWarning")}>{`\n${outdated}`}</Text> : null}
    </Text>
  );
}

function Message({ theme, message, colour }: { theme: PluginTheme; message: string; colour?: "statusDanger" | "statusWarning" }) {
  return <Text style={[text(theme, "meta", colour), { padding: SPACING[4] }]}>{message}</Text>;
}

/**
 * What the user folded is kept in the host's "piggery-view" setting: read once when it arrives, saved a
 * moment after the last change (a burst of clicks is one save), only teams piggery still lists.
 * `change` shows a new state at once and saves it.
 */
function useFolds(set: (f: Folds) => void, ps: Ps | null) {
  const stored = useSettings(viewSettings);
  const latest = useRef({ stored, ps });
  latest.current = { stored, ps };
  const restored = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => {
    if (stored.status === "ready" && !restored.current) {
      restored.current = true;
      set(foldsOf(stored.values));
    }
  }, [stored.status]);
  useEffect(() => () => clearTimeout(timer.current), []);
  const change = (next: Folds) => {
    set(next);
    clearTimeout(timer.current);
    timer.current = setTimeout(async () => {
      const { stored: s, ps: p } = latest.current;
      if (s.status !== "ready") return;
      const ids = [...(p?.teams ?? []), ...(p?.closed ?? [])].map((t) => t.id);
      if (!(await s.save(prune(next, ids), s.revision))) await s.reload();
    }, 300);
  };
  return { change };
}

/**
 * piggery top's table in Paseo's finish: per project directory a small label, then its teams (rows
 * that fold their member tree; closed teams folded) and solos; then the latest events. `dir` limits
 * it to one workspace. A selected row opens beside the list when there is room, otherwise in its place.
 */
function Farm({ theme, compact, dir, empty }: { theme: PluginTheme; compact: boolean; dir?: string; empty: string }) {
  const { value: snap, error } = useSnapshot(dir);
  const ps = snap?.ps ?? null;
  const [width, setWidth] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const [folds, setFolds] = useState<Folds>(NO_FOLDS);
  const { change } = useFolds(setFolds, ps);
  const projects = ps ? farm(ps, dir === undefined ? undefined : snap?.dir) : [];
  const rows = new Map(projects.flatMap((p) => p.units.flatMap((u) => u.rows.map((r) => [r.id, r] as const))));
  const pick = rows.get(selected ?? "") ?? null;
  const beside = !compact && width >= SPLIT_MIN;
  const listWidth = pick && beside ? width / 2 : width;
  const grid: Grid = { width: listWidth, cwd: ps ? hasCwd(ps) : true };
  const select = (id: string) => setSelected(id === selected ? null : id);

  const list = (
    <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: SPACING[6] }}>
      {error ? <Message theme={theme} message={error} colour="statusDanger" /> : null}
      {ps && ps.protocol_version !== PROTOCOL_VERSION ? (
        <Message theme={theme} colour="statusWarning" message={`piggery speaks protocol ${ps.protocol_version ?? "?"}; this plugin was written for ${PROTOCOL_VERSION}. Some fields may be missing.`} />
      ) : null}
      {!ps && !error ? <Message theme={theme} message="Loading…" /> : null}
      {ps && projects.length === 0 ? <Message theme={theme} message={empty} /> : null}
      {ps && dir === undefined ? <Status theme={theme} ps={ps} outdated={snap?.outdated ?? ""} /> : null}
      {projects.length > 0 ? <Header theme={theme} grid={grid} /> : null}
      {projects.map((project) => (
        <View key={project.path}>
          <GroupLabel theme={theme} label={project.title} icon="Folder" />
          <Rows theme={theme}>
            {project.units.map((unit) => {
              if (unit.kind === "solo") {
                const row = unit.rows[0];
                return <MemberRow key={unit.id} theme={theme} row={row} grid={grid} indent={CHEVRON_SLOT} selected={row.id === selected} onSelect={() => select(row.id)} />;
              }
              const open = teamOpen(folds, unit.id, unit.open);
              const goneOpen = goneIsOpen(folds, unit.id);
              return [
                unit.kind === "closed" || unit.dead ? (
                  <DeadTeamRow key={unit.id} theme={theme} unit={unit} grid={grid} open={open} onToggle={() => change(setTeam(folds, unit.id, !open, unit.open))} />
                ) : (
                  <TeamRow key={unit.id} theme={theme} unit={unit} open={open} onToggle={() => change(setTeam(folds, unit.id, !open, unit.open))} />
                ),
                open && unit.rows.length === 0 ? (
                  <ListRow key={`${unit.id}-none`} theme={theme}>
                    <Text style={[text(theme, "meta"), { paddingLeft: CHEVRON_SLOT }]}>No members. Open an agent session in its directory and ask the gate to admit it.</Text>
                  </ListRow>
                ) : null,
                ...(open ? unit.rows.filter((row) => !row.folded || goneOpen).map((row) => <MemberRow key={row.id} theme={theme} row={row} grid={grid} indent={CHEVRON_SLOT + row.depth * INDENT} selected={row.id === selected} onSelect={() => select(row.id)} />) : []),
                open && unit.goneLine ? <GoneRow key={`${unit.id}-gone`} theme={theme} line={unit.goneLine} grid={grid} open={goneOpen} onToggle={() => change(setGone(folds, unit.id, !goneOpen))} /> : null,
              ];
            })}
          </Rows>
        </View>
      ))}
      {ps && dir === undefined ? <Events theme={theme} ps={ps} grid={grid} folded={!folds.eventsOpen} onToggle={() => change(setEvents(folds, !folds.eventsOpen))} /> : null}
    </ScrollView>
  );

  return (
    <View style={{ flex: 1, flexDirection: "row", backgroundColor: theme.colors.surface0 }} onLayout={(e) => setWidth(e.nativeEvent.layout.width)}>
      {pick && !beside ? null : list}
      {pick ? (
        <View style={{ flex: 1, borderLeftWidth: beside ? 1 : 0, borderLeftColor: theme.colors.border }}>
          <Detail key={pick.id} theme={theme} row={pick} beside={beside} onClose={() => setSelected(null)} />
        </View>
      ) : null}
    </View>
  );
}

/** Sidebar surface: every project, like piggery top. */
export function PiggerySurface({ theme, layout }: PluginSurfaceProps) {
  return <Farm theme={theme} compact={layout.compact} empty="No piggery team or session is open." />;
}

/** Workspace panel: the teams and sessions of this workspace's directory. */
export function FarmPanel({ theme, layout, workspaceId }: PluginWorkspacePanelProps) {
  const dir = useWorkspace(workspaceId, (w) => w.directory);
  if (!dir) return <View style={{ flex: 1, backgroundColor: theme.colors.surface0 }} />;
  return <Farm theme={theme} compact={layout.compact} dir={dir} empty="No team works here yet. Ask your agent: “make a supervisor-executor team for …”" />;
}
