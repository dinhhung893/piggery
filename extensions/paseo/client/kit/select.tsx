import type { PluginTheme } from "@getpaseo/plugin";
import { Icon, Modal } from "@getpaseo/plugin/client/react-native";
import { useState } from "react";
import { Text, View } from "react-native";
import { PressRow } from "./press.tsx";
import { ICON_SIZE, RADIUS, ROW_MIN, SPACING, text } from "./theme.ts";

export interface Option {
  value: string;
  label: string;
  count?: number;
}

/**
 * One filter as one control, however many options there are: its label and the chosen option on a
 * bordered button; pressing it lists the options in the host's dialog, the chosen one ticked.
 */
export function Select({ theme, label, value, options, onPick }: { theme: PluginTheme; label: string; value: string; options: Option[]; onPick: (value: string) => void }) {
  const [open, setOpen] = useState(false);
  const chosen = options.find((o) => o.value === value);
  return (
    <>
      <PressRow theme={theme} onPress={() => setOpen(true)} label={`${label}: ${chosen?.label ?? ""}`} style={{ flexDirection: "row", alignItems: "center", gap: SPACING[1.5], paddingHorizontal: SPACING[2], paddingVertical: SPACING[1], borderRadius: RADIUS.md, borderWidth: 1, borderColor: theme.colors.border, flexShrink: 1 }}>
        <Text style={text(theme, "label")}>{label}</Text>
        <Text style={[text(theme, "label", "foreground"), { flexShrink: 1 }]} numberOfLines={1}>
          {chosen?.label ?? ""}
        </Text>
        <Icon name="ChevronDown" size={ICON_SIZE.sm} color={theme.colors.foregroundMuted} />
      </PressRow>
      <Modal title={label} open={open} onOpenChange={setOpen}>
        <Modal.Content contentContainerStyle={{ padding: SPACING[2], gap: 0 }}>
          {options.map((o) => (
            <PressRow key={o.value} theme={theme} onPress={() => { onPick(o.value); setOpen(false); }} label={o.label} style={{ flexDirection: "row", alignItems: "center", gap: SPACING[2], minHeight: ROW_MIN, paddingHorizontal: SPACING[2], borderRadius: RADIUS.md }}>
              <View style={{ width: ICON_SIZE.sm }}>{o.value === value ? <Icon name="Check" size={ICON_SIZE.sm} color={theme.colors.foreground} /> : null}</View>
              <Text style={[text(theme, "rowTitle"), { flex: 1, minWidth: 0 }]} numberOfLines={1}>
                {o.label}
              </Text>
              {o.count === undefined ? null : <Text style={[text(theme, "label"), { fontVariant: ["tabular-nums"] }]}>{o.count}</Text>}
            </PressRow>
          ))}
        </Modal.Content>
      </Modal>
    </>
  );
}
