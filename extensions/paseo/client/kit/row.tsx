// A participant as one line, however many there are: a table across the line's width. Its columns are
// shares of it (theme.ts SHARE), so they line up down the list for every kind of line (members, solos,
// gone lines, team lines): the lead (the thin bar in the state's colour, the name, the `gate` tag) takes
// what the tail leaves, and the tail (role, state, ctx, since) is a fixed fraction of the line. The
// name shortens before its tag does. Narrow, the role goes first, then ctx (see columnsFor). Everything
// else (harness, model, turns, age, unacked) is in the row's dialog.
import type { PluginTheme } from "@getpaseo/plugin";
import type { ReactNode } from "react";
import { Text, View } from "react-native";
import type { Row } from "../../shared/view.ts";
import { StateWord, statusColour } from "./badge.tsx";
import { PressRow } from "./press.tsx";
import { BAR, BOTH_MIN, CTX_MIN, NAME_MIN, ROW_ONE, SHARE, SPACING, STATE_MIN, text } from "./theme.ts";

/** Which muted columns a line this wide (px) has room for: the role goes first, then ctx. */
export type Columns = "both" | "ctx" | "none";
export const columnsFor = (width: number): Columns => (width >= BOTH_MIN ? "both" : width >= CTX_MIN ? "ctx" : "none");

/** The tail's width as a share of the line: the same for every line, so its columns start at one x. */
function tailWidth(columns: Columns, state: boolean): `${number}%` {
  const tail = (state ? SHARE.state : 0) + SHARE.since + (columns !== "none" ? SHARE.ctx : 0) + (columns === "both" ? SHARE.role : 0);
  return `${(100 * tail) / (SHARE.name + tail)}%`;
}

/** The left part of a line: `left` px of indent (and the chevron's room), then what the caller puts, then the gap every column has, so a tag never touches the state; never narrower than the name needs. */
export function Lead({ left, children }: { left: number; children: ReactNode }) {
  return <View style={{ flex: 1, minWidth: left + NAME_MIN, flexDirection: "row", alignItems: "center", gap: SPACING[2], paddingLeft: left, paddingRight: SPACING[2], overflow: "hidden" }}>{children}</View>;
}

/** The columns after the name: role, state, ctx, since (a line without a role or ctx leaves its cells empty). */
export function Tail({ theme, row, columns, dim, noState }: { theme: PluginTheme; row: Row; columns: Columns; dim?: boolean; /** The state is said elsewhere (a Board column's heading): leave its cell out. */ noState?: boolean }) {
  const num = { fontVariant: ["tabular-nums" as const] };
  const cell = { minWidth: 0 };
  return (
    <View style={{ width: tailWidth(columns, !noState), flexShrink: 0, flexDirection: "row", alignItems: "center", gap: SPACING[2] }}>
      {columns === "both" ? <Text style={[text(theme, "label"), cell, { flex: SHARE.role }]} numberOfLines={1}>{row.role ?? ""}</Text> : null}
      {noState ? null : (
        <View style={{ flex: SHARE.state, minWidth: STATE_MIN }}>
          <StateWord theme={theme} status={row.status} word={row.state_text} dim={dim ?? row.dim === true} />
        </View>
      )}
      {columns !== "none" ? <Text style={[text(theme, "label"), cell, num, { flex: SHARE.ctx, textAlign: "right" }]} numberOfLines={1}>{row.ctx ?? ""}</Text> : null}
      <Text style={[text(theme, "label"), cell, num, { flex: SHARE.since, textAlign: "right" }]} numberOfLines={1}>{row.since ?? ""}</Text>
    </View>
  );
}

/**
 * The Overview's participant line; the Board's card is the same with `columns` "none", no indent and no
 * slot. `slot` is what stays free at the right edge (the room of a line's info button), the same for
 * every kind of line at a width.
 */
export function AgentRow({ theme, row, left, slot, columns, noState, onPress }: { theme: PluginTheme; row: Row; left: number; slot: number; columns: Columns; noState?: boolean; onPress: () => void }) {
  const dim = row.dim === true;
  return (
    <PressRow theme={theme} onPress={onPress} label={`${row.name}, ${row.state_text}`} style={{ flexDirection: "row", minHeight: ROW_ONE }}>
      <View style={{ flex: 1, minWidth: 0, flexDirection: "row" }}>
        <Lead left={left}>
          <View style={{ width: BAR, alignSelf: "stretch", marginVertical: SPACING[1.5], borderRadius: BAR, backgroundColor: statusColour(theme, row.status, dim) }} />
          <Text style={[text(theme, "rowTitle", dim ? "foregroundMuted" : "foreground"), { flexShrink: 1, minWidth: 0 }]} numberOfLines={1}>
            {row.name}
          </Text>
          {row.gate ? <Text style={[text(theme, "label"), { flexShrink: 0 }]}>gate</Text> : null}
        </Lead>
        <Tail theme={theme} row={row} columns={columns} noState={noState} />
      </View>
      <View style={{ width: slot }} />
    </PressRow>
  );
}
