import {
  MAX_BYTES,
  MAX_DURATION_MS,
  hasAcceptedExtension,
  type Rejection,
} from "./limits";

/**
 * Reads a file's duration in the browser, by loading its metadata into a
 * detached `<audio>` element.
 *
 * `preload="metadata"` is what keeps this cheap: the browser reads the header
 * rather than the whole file.
 */
export function readDuration(file: File): Promise<number | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const audio = document.createElement("audio");
    audio.preload = "metadata";

    const done = (durationMs: number | null) => {
      URL.revokeObjectURL(url);
      resolve(durationMs);
    };

    audio.onloadedmetadata = () => {
      const seconds = audio.duration;
      done(Number.isFinite(seconds) && seconds > 0 ? Math.round(seconds * 1000) : null);
    };
    audio.onerror = () => done(null);
    audio.src = url;
  });
}

/**
 * The §11.1 pre-check: type, then size, then duration, in that order so an
 * oversized file is refused before its metadata is read.
 */
export async function precheck(file: File): Promise<Rejection | null> {
  const about = { name: file.name, bytes: file.size };
  if (!hasAcceptedExtension(file.name)) return { ...about, reason: "type" };
  if (file.size > MAX_BYTES) return { ...about, reason: "size" };

  const durationMs = await readDuration(file);
  if (durationMs === null) return null;
  if (durationMs > MAX_DURATION_MS) return { ...about, reason: "duration", durationMs };
  return null;
}
