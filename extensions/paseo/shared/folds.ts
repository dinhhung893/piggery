// What the user chose in the surface, remembered by Paseo (the plugin's "piggery-view" host setting;
// not piggery top's): the folds that differ from piggery's defaults, keyed by team id, and the tab. Ids
// of teams that are gone from piggery are dropped when saving. No imports, so `node --test` loads it as is.

export type SurfaceTab = "overview" | "board";

export interface Folds {
  /** A team's open state, when the user chose one other than ps --view's default for it. */
  teams: Record<string, boolean>;
  /** Teams whose gone-members line the user expanded (the default is folded). */
  gone: Record<string, boolean>;
  /** The surface's tab. */
  tab: SurfaceTab;
}

export const NO_FOLDS: Folds = { teams: {}, gone: {}, tab: "overview" };

/** Folds as the host stored them, tolerating a missing or odd document. */
export function foldsOf(values: Partial<Folds> | null | undefined): Folds {
  return { teams: { ...values?.teams }, gone: { ...values?.gone }, tab: values?.tab === "board" ? "board" : "overview" };
}

export const teamOpen = (f: Folds, id: string, byDefault: boolean): boolean => f.teams[id] ?? byDefault;
export const goneOpen = (f: Folds, id: string): boolean => f.gone[id] ?? false;

/** Set a team open or closed; choosing its default forgets the entry. */
export function setTeam(f: Folds, id: string, open: boolean, byDefault: boolean): Folds {
  const { [id]: _, ...rest } = f.teams;
  return { ...f, teams: open === byDefault ? rest : { ...rest, [id]: open } };
}

export function setGone(f: Folds, id: string, open: boolean): Folds {
  const { [id]: _, ...rest } = f.gone;
  return { ...f, gone: open ? { ...rest, [id]: true } : rest };
}

export const setTab = (f: Folds, tab: SurfaceTab): Folds => ({ ...f, tab });

/** Only the entries of teams that still exist. */
export function prune(f: Folds, ids: Iterable<string>): Folds {
  const live = new Set(ids);
  const keep = <T>(m: Record<string, T>) => Object.fromEntries(Object.entries(m).filter(([id]) => live.has(id)));
  return { teams: keep(f.teams), gone: keep(f.gone), tab: f.tab };
}
