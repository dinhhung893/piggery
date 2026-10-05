import type { PluginTheme } from "@getpaseo/plugin";
import { Text, View } from "react-native";
import type { Daemon, Summary } from "../shared/view.ts";
import { DOT, SPACING, text } from "./kit/theme.ts";
import { GRACE_MS, pollEvery } from "./poll.ts";

type Colour = keyof PluginTheme["colors"];

/** top's header counts, in top's order; the colour is the status's where it has one. Zero counts are left out. */
const STATS: { key: "working" | "idle" | "waiting" | "held" | "unacked"; colour?: Colour }[] = [
  { key: "working", colour: "statusSuccess" },
  { key: "idle" },
  { key: "waiting", colour: "statusWarning" },
  { key: "held" },
  { key: "unacked" },
];

/** The live statuses drawn as one bar, each as wide as its count (gone is not in it), in the counts' order so a colour sits under its number. */
const BAR: { key: "working" | "waiting" | "idle"; colour: Colour }[] = [
  { key: "working", colour: "statusSuccess" },
  { key: "idle", colour: "foregroundMuted" },
  { key: "waiting", colour: "statusWarning" },
];

/**
 * The surface's head, from ps --view's summary and daemon: each count as a block (the number over its
 * word; a block never splits, a row that does not fit wraps whole blocks), a thin bar of the live
 * statuses, then one muted line: the teams, the daemon's age, when this was read (a dot before it:
 * green while the last read is within the poll interval, amber once it is overdue) and, amber, a
 * version that does not match. What needs `piggery setup` follows in amber.
 */
export function Header({ theme, summary, daemon, at, now }: { theme: PluginTheme; summary: Summary; daemon: Daemon | undefined; at: number | null; now: number }) {
  const count = (key: (typeof STATS)[number]["key"]) => summary[key] ?? 0;
  const stats = STATS.filter((s) => count(s.key) > 0);
  const live = BAR.reduce((n, b) => n + count(b.key), 0);
  const fresh = at !== null && now - at <= pollEvery(count("working") + count("waiting") > 0) + GRACE_MS;
  const note = daemon?.version_mismatch ? daemon.version_notes?.[0] : undefined;
  const line = [`${summary.teams} team${summary.teams === 1 ? "" : "s"}`, daemon && `daemon ${daemon.age} up`, at !== null && `read ${Math.max(0, Math.round((now - at) / 1000))}s ago`].filter(Boolean).join(" · ");
  return (
    <View style={{ gap: SPACING[3] }}>
      {stats.length > 0 ? (
        <View style={{ flexDirection: "row", flexWrap: "wrap", columnGap: SPACING[6], rowGap: SPACING[2] }}>
          {stats.map((s) => (
            <View key={s.key} accessibilityLabel={`${count(s.key)} ${s.key}`}>
              <Text style={[text(theme, "stat", s.colour ?? "foreground"), { fontVariant: ["tabular-nums"] }]}>{count(s.key)}</Text>
              <Text style={text(theme, "label")}>{s.key}</Text>
            </View>
          ))}
        </View>
      ) : null}
      {live > 0 ? (
        <View style={{ flexDirection: "row", gap: 2, height: 4, borderRadius: 2, overflow: "hidden" }}>
          {BAR.filter((b) => count(b.key) > 0).map((b) => (
            <View key={b.key} style={{ flex: count(b.key), backgroundColor: theme.colors[b.colour] }} />
          ))}
        </View>
      ) : null}
      <View style={{ flexDirection: "row", alignItems: "flex-start", gap: SPACING[1.5] }}>
        {at !== null ? <View accessibilityLabel={fresh ? "up to date" : "overdue"} style={{ width: DOT, height: DOT, borderRadius: DOT, marginTop: 4, backgroundColor: fresh ? theme.colors.statusSuccess : theme.colors.statusWarning }} /> : null}
        <Text style={[text(theme, "label"), { flex: 1, minWidth: 0 }]}>
          {line}
          {note ? <Text style={text(theme, "label", "statusWarning")}>{` · ${note}`}</Text> : null}
        </Text>
      </View>
      {daemon?.outdated ? <Text style={text(theme, "label", "statusWarning")}>{daemon.outdated}</Text> : null}
    </View>
  );
}
