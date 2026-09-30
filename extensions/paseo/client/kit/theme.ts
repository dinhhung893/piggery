// The host's look, copied: the plugin SDK (0.9.1) exports only colours (PluginTheme). The layout is
// piggery top's table; its finish is Paseo's (Settings → Providers: small muted labels, rows split by
// dividers in the border colour, the status dot and its word). Sources, in Paseo packages/app/src,
// read 2026-09-28:
//   styles/theme.ts:544-600            SPACING, FONT_SIZE, ICON_SIZE, FONT_WEIGHT, BORDER_RADIUS
//   styles/settings.ts:4-58            section, sectionHeader(Title), card, row, rowBorder, rowTitle, rowHint
//   screens/settings/providers-section.tsx:476-545  row minHeight/gap, hover, status dot, statusLabel, separator
//   components/ui/control-geometry.ts:25-31,225-253  segmented control (sm): heights, inset, padding, radius
//   components/ui/segmented-control.tsx:167-230      segmented gap, selected/hover background, label colours
// Colours always come from the theme prop, never from here.
import type { PluginTheme } from "@getpaseo/plugin";
import type { TextStyle } from "react-native";

/** theme.ts:544 */
export const SPACING = { 0.5: 2, 1: 4, 1.5: 6, 2: 8, 3: 12, 4: 16, 6: 24, 8: 32 } as const;

/** theme.ts:561 */
const FONT_SIZE = { code: 12, sm: 12, base: 14, lg: 16 } as const;

/** theme.ts:577 */
export const ICON_SIZE = { sm: 14, md: 16 } as const;

/** theme.ts:591 */
export const RADIUS = { md: 6, lg: 8 } as const;

/** control-geometry.ts:27 FIELD_CONTROL_HEIGHT: a list row's least height, dense enough for a table. */
export const ROW_MIN = 44;

/** providers-section.tsx:522: the status dot. */
export const DOT = 8;

/** control-geometry.ts:26,30,238-242: a segmented control's height and a segment's. */
export const SEGMENT = { height: 32, inset: 2, paddingX: 8 } as const;

/** One tree level's indent. */
export const INDENT = SPACING[4];

/** A disclosure's chevron and the gap after it: what a team's members are indented by, so their marks start under the team's icon. */
export const CHEVRON_SLOT = ICON_SIZE.sm + SPACING[2];

/** top's columns, in its order, with their widths; the name takes what is left. */
export const COLUMNS = [
  { key: "role", label: "Role", width: 96 },
  { key: "state", label: "State", width: 108 },
  { key: "harness", label: "Harness", width: 88 },
  { key: "model", label: "Model", width: 140 },
  { key: "ctx", label: "Ctx", width: 56 },
  { key: "turns", label: "Turns", width: 52 },
  { key: "age", label: "Age", width: 48 },
  { key: "since", label: "Since", width: 48 },
  { key: "cwd", label: "Cwd", width: 120 },
] as const;

export type ColumnKey = (typeof COLUMNS)[number]["key"];

/** Narrower than these list widths, a column hides (never wraps): cwd first, the state last. */
export const HIDE_BELOW: Record<ColumnKey, number> = { cwd: 1100, age: 1000, turns: 940, ctx: 880, harness: 800, role: 720, model: 600, since: 500, state: 0 };

/** A column of numbers: right-aligned. */
export const numeric = (key: ColumnKey) => key === "ctx" || key === "turns" || key === "age" || key === "since";

/** The columns that fit a list this wide, in top's order; `cwd` false leaves Cwd out (no row has one). */
export function columnsFor(width: number, cwd = true) {
  return COLUMNS.filter((c) => width >= HIDE_BELOW[c.key] && (cwd || c.key !== "cwd"));
}

/**
 * An event's fields sit in the member columns they fall under, whichever columns the list width shows:
 * its type under Role and State, its target under Harness to Turns, its time under Age and Since.
 * Together the slots cover the shown columns in order, so each event field starts and ends where a
 * member column does.
 */
export const EVENT_SLOTS: { what: "type" | "target" | "time" | "rest"; keys: ColumnKey[] }[] = [
  { what: "type", keys: ["role", "state"] },
  { what: "target", keys: ["harness", "model", "ctx", "turns"] },
  { what: "time", keys: ["age", "since"] },
  { what: "rest", keys: ["cwd"] },
];

/** The width of the columns of `keys` that a list this wide shows; 0 when none is. */
export function slotWidth(width: number, keys: ColumnKey[], cwd = true): number {
  return columnsFor(width, cwd).filter((c) => keys.includes(c.key)).reduce((sum, c) => sum + c.width, 0);
}

/** A left-aligned column whose left neighbour, among the shown ones, is right-aligned: it gets room (Cell `apart`). */
export function apart(width: number, key: ColumnKey, cwd = true): boolean {
  const cols = columnsFor(width, cwd);
  const i = cols.findIndex((c) => c.key === key);
  return i > 0 && !numeric(key) && numeric(cols[i - 1].key);
}

/** The name column's least width. */
export const NAME_MIN = 140;

/** Wider than this, a selected row opens beside the list; narrower, it replaces the list. */
export const SPLIT_MIN = 900;

type Colour = keyof PluginTheme["colors"];

const ROLES = {
  /** settings.ts:14 sectionHeaderTitle: project directories, column labels, Events. */
  label: { fontSize: FONT_SIZE.sm, lineHeight: 16, fontWeight: "400", colour: "foregroundMuted" },
  /** settings.ts:50 rowTitle: a row's name. */
  rowTitle: { fontSize: FONT_SIZE.base, lineHeight: 20, fontWeight: "400", colour: "foreground" },
  /** A selected row's name at the top of the detail. */
  detailTitle: { fontSize: FONT_SIZE.lg, lineHeight: 22, fontWeight: "500", colour: "foreground" },
  /** providers-section.tsx:526 statusLabel and separator: state word and the " · " fields. */
  meta: { fontSize: FONT_SIZE.base, lineHeight: 20, fontWeight: "400", colour: "foregroundMuted" },
    /** Tail lines, at the host's code size. */
  code: { fontSize: FONT_SIZE.code, lineHeight: 18, fontWeight: "400", colour: "foreground", fontFamily: "monospace" },
} as const satisfies Record<string, TextStyle & { colour: Colour }>;

export type Role = keyof typeof ROLES;

/** The style of a text role; `colour` swaps only its colour. */
export function text(theme: PluginTheme, role: Role, colour?: Colour): TextStyle {
  const { colour: base, ...style } = ROLES[role];
  return { ...style, color: theme.colors[colour ?? base] };
}
