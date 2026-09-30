// What the user folded in the list, remembered by Paseo (the plugin's "piggery-view" host setting;
// not piggery top's). Only what differs from the defaults is kept, keyed by team id, and ids of teams
// that are gone from piggery are dropped when saving. No imports, so `node --test` loads it as is.

export interface Folds {
  /** A team's open state, when the user chose one other than the default (live teams open; dead and closed folded). */
  teams: Record<string, boolean>;
  /** Teams whose gone-members line the user expanded (the default is folded). */
  gone: Record<string, boolean>;
  /** The user opened the Events list (the default is folded to its latest line). Not the old `events` key, which meant folded: it is ignored. */
  eventsOpen: boolean;
}

export const NO_FOLDS: Folds = { teams: {}, gone: {}, eventsOpen: false };

/** Folds as the host stored them, tolerating a missing or odd document. */
export function foldsOf(values: Partial<Folds> | null | undefined): Folds {
  return { teams: { ...values?.teams }, gone: { ...values?.gone }, eventsOpen: values?.eventsOpen === true };
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

export const setEvents = (f: Folds, open: boolean): Folds => ({ ...f, eventsOpen: open });

/** Only the entries of teams that still exist. */
export function prune(f: Folds, ids: Iterable<string>): Folds {
  const live = new Set(ids);
  const keep = <T>(m: Record<string, T>) => Object.fromEntries(Object.entries(m).filter(([id]) => live.has(id)));
  return { teams: keep(f.teams), gone: keep(f.gone), eventsOpen: f.eventsOpen };
}
