const places = new Map<string, number>();
let era = 0;
let sounding: HTMLMediaElement | null = null;

/**
 * placeEra names the memory of places open now. forgetAllPlaces closes it
 * and opens the next, so a player that takes the era as it mounts can tell,
 * on its way out, whether the attempt it kept places for is still open.
 */
export function placeEra(): number {
  return era;
}

/**
 * keepPlace remembers, in memory for the tab's life, that the play in
 * progress under `key` stopped at `seconds`, so a player that comes back
 * resumes there without spending a play (spec §11.4). A reload forgets it.
 * Given `since`, an era from placeEra, it keeps nothing once forgetAllPlaces
 * has closed that era: a player that leaves with its attempt, whose page
 * forgets every place before the player's own cleanup runs, cannot put a
 * place back for the next attempt.
 */
export function keepPlace(key: string, seconds: number, since: number = era): void {
  if (since !== era) return;
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

/**
 * forgetAllPlaces drops every remembered place, as the attempt they belong
 * to closes, and opens the next era (placeEra).
 */
export function forgetAllPlaces(): void {
  places.clear();
  era += 1;
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
