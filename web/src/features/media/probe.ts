import type { components } from "@/lib/api/schema";
import {
  MAX_DURATION_MS,
  hasAcceptedExtension,
  maxBytes,
  type Rejection,
} from "./limits";

type MediaKind = components["schemas"]["MediaKind"];

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
 * quickCheck is the part of the pre-check that reads nothing of the file: the
 * extensions the kind takes, then the kind's size limit (DG-63).
 */
export function quickCheck(file: File, kind: MediaKind = "audio"): Rejection | null {
  const about = { name: file.name, bytes: file.size, kind };
  if (!hasAcceptedExtension(file.name, kind)) return { ...about, reason: "type" };
  if (file.size > maxBytes(kind)) return { ...about, reason: "size" };
  return null;
}

/**
 * The §11.1 pre-check: type, then size, then, for audio, duration, in that
 * order so an oversized file is refused before its metadata is read.
 */
export async function precheck(
  file: File,
  kind: MediaKind = "audio",
): Promise<Rejection | null> {
  const quick = quickCheck(file, kind);
  if (quick !== null || kind === "image") return quick;

  const durationMs = await readDuration(file);
  if (durationMs === null) return null;
  if (durationMs > MAX_DURATION_MS)
    return { name: file.name, bytes: file.size, kind, reason: "duration", durationMs };
  return null;
}
