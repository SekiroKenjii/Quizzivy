import type { components } from "@/lib/api/schema";

type MediaKind = components["schemas"]["MediaKind"];

/** §11.1's limits, mirrored client-side so the teacher is told before uploading. */
export const MAX_AUDIO_BYTES = 50 * 1024 * 1024;
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024;
export const MAX_DURATION_MS = 5 * 60 * 1000;

/** The extensions the file picker offers for audio. The server decides by magic bytes. */
export const ACCEPTED_EXTENSIONS = [".mp3", ".m4a"] as const;
export const ACCEPT_ATTRIBUTE = ".mp3,.m4a,audio/mpeg,audio/mp4";

/** IMAGE_EXTENSIONS is what the picker offers for an image (DG-63): no GIF. */
export const IMAGE_EXTENSIONS = [".png", ".jpg", ".jpeg", ".webp"] as const;
export const IMAGE_ACCEPT_ATTRIBUTE =
  ".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp";

export type RejectionReason = "type" | "size" | "duration" | "unreadable";

export interface Rejection {
  reason: RejectionReason;
  /** The deck's A-05 rule: a rejection names the file and the reason together. */
  name: string;
  bytes: number;
  /** The kind the file was checked as, so the message names that kind's rule. */
  kind?: MediaKind;
  /** Filled for a duration rejection, so the message can name the length. */
  durationMs?: number;
}

const EXTENSIONS: Record<MediaKind, readonly string[]> = {
  audio: ACCEPTED_EXTENSIONS,
  image: IMAGE_EXTENSIONS,
};

/** hasAcceptedExtension reports whether the name ends in an extension the kind takes; audio by default. */
export function hasAcceptedExtension(name: string, kind: MediaKind = "audio"): boolean {
  const lower = name.toLowerCase();
  return EXTENSIONS[kind].some((ext) => lower.endsWith(ext));
}

/** acceptAttribute is the file input's `accept` for a kind. */
export function acceptAttribute(kind: MediaKind): string {
  return kind === "image" ? IMAGE_ACCEPT_ATTRIBUTE : ACCEPT_ATTRIBUTE;
}

/** maxBytes is the most a file of the kind may weigh. */
export function maxBytes(kind: MediaKind): number {
  return kind === "image" ? MAX_IMAGE_BYTES : MAX_AUDIO_BYTES;
}

/** kindOfName guesses a file's kind from its extension, or null when no kind takes it. */
export function kindOfName(name: string): MediaKind | null {
  if (hasAcceptedExtension(name, "audio")) return "audio";
  if (hasAcceptedExtension(name, "image")) return "image";
  return null;
}
