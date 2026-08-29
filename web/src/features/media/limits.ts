/** §11.1's limits, mirrored client-side so the teacher is told before uploading. */
export const MAX_BYTES = 10 * 1024 * 1024;
export const MAX_DURATION_MS = 5 * 60 * 1000;

/** The extensions the file picker offers. The server decides by magic bytes. */
export const ACCEPTED_EXTENSIONS = [".mp3", ".m4a"] as const;
export const ACCEPT_ATTRIBUTE = ".mp3,.m4a,audio/mpeg,audio/mp4";

export type RejectionReason = "type" | "size" | "duration" | "unreadable";

export interface Rejection {
  reason: RejectionReason;
  /** Filled for a duration rejection, so the message can name the length. */
  durationMs?: number;
}

export function hasAcceptedExtension(name: string): boolean {
  const lower = name.toLowerCase();
  return ACCEPTED_EXTENSIONS.some((ext) => lower.endsWith(ext));
}
