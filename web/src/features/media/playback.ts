const places = new Map<string, number>();
let sounding: HTMLMediaElement | null = null;

/**
 * keepPlace remembers, in memory for the tab's life, that the play in
 * progress under `key` stopped at `seconds`, so a player that comes back
 * resumes there without spending a play (spec §11.4). A reload forgets it.
 */
export function keepPlace(key: string, seconds: number): void {
  places.set(key, seconds);
}

/** recallPlace returns where the play in progress under `key` stopped, or null for none. */
export function recallPlace(key: string): number | null {
  return places.get(key) ?? null;
}

/** forgetPlace drops the play in progress under `key`: it ended, or its place could not be put back. */
export function forgetPlace(key: string): void {
  places.delete(key);
}

/** forgetAllPlaces drops every remembered place, as the attempt they belong to closes. */
export function forgetAllPlaces(): void {
  places.clear();
}

/**
 * takeTurn makes `element` the one player sounding, pausing the one that was
 * before it. A player calls it as its element starts, and releaseTurn as it
 * goes away.
 */
export function takeTurn(element: HTMLMediaElement): void {
  if (sounding !== null && sounding !== element && !sounding.paused) sounding.pause();
  sounding = element;
}

/** releaseTurn forgets `element` as the player sounding, if it is. */
export function releaseTurn(element: HTMLMediaElement): void {
  if (sounding === element) sounding = null;
}
