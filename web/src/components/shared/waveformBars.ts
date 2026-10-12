/** waveformSeed converts an asset id to a stable FNV-1a integer and preserves numeric deck indexes. */
export function waveformSeed(seed: string | number): number {
  if (typeof seed === "number") return Number.isFinite(seed) ? seed : 0;
  let hash = 2166136261;
  for (let index = 0; index < seed.length; index++) {
    hash = Math.imul(hash ^ seed.charCodeAt(index), 16777619);
  }
  return hash >>> 0;
}

/** waveformBars returns the deck's 28 deterministic bar heights for a card seed. */
export function waveformBars(seed: number): readonly number[] {
  return Array.from(
    { length: 28 },
    (_, index) => 20 + ((((index * 37 + seed * 11) % 70) + 70) % 70),
  );
}
