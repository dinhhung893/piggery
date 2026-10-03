import type { PluginServerContext } from "@getpaseo/plugin/server";
import { piggeryPath } from "./server/installed.ts";
import { pickBin, realDir, runPiggery } from "./server/piggery.ts";
import { settings, snapshot, tail, viewSettings } from "./shared/rpc.ts";
import { TAIL_VERSION } from "./shared/view.ts";

export default function contribute(server: PluginServerContext) {
  const stored = server.registerSettings(settings);
  server.registerSettings(viewSettings);
  const bin = async () => {
    const state = await stored.read();
    return pickBin(state.status === "ready" ? state.values.path : "", piggeryPath);
  };

  server.handle(snapshot, async ({ dir }) => {
    const b = await bin();
    const r = await runPiggery(b, ["ps", "--view"]);
    if (!r.ok) return r;
    try {
      const view = JSON.parse(r.out) as Record<string, unknown>;
      return { ok: true as const, view, dir: dir === undefined ? undefined : await realDir(dir) };
    } catch {
      return { ok: false as const, code: "failed" as const, error: "piggery ps --view did not print JSON." };
    }
  });

  server.handle(tail, async ({ id, lines }) => {
    const r = await runPiggery(await bin(), ["tail", id, "-n", String(lines), "--view"]);
    if (!r.ok) return r;
    try {
      const doc = JSON.parse(r.out);
      if (doc.version < TAIL_VERSION) return { ok: false as const, code: "older" as const, error: `piggery speaks tail --view version ${doc.version}; this plugin reads ${TAIL_VERSION}. Update piggery.` };
      if (doc.version > TAIL_VERSION) return { ok: false as const, code: "newer" as const, error: `piggery speaks tail --view version ${doc.version}; this plugin reads ${TAIL_VERSION}. Run piggery setup paseo to update the plugin, then reload the Paseo app.` };
      if (Array.isArray(doc.lines)) return { ok: true as const, lines: doc.lines };
    } catch {}
    return { ok: false as const, code: "failed" as const, error: "piggery tail --view did not print JSON." };
  });

  return () => {};
}
