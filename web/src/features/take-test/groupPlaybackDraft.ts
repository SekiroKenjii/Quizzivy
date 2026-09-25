import { z } from "zod";

const prefix = "quizzivy.group-play.";
const schema = z.object({
  studentId: z.string(),
  attemptId: z.string(),
  deadlineAt: z.number(),
  playId: z.uuid(),
  recordingId: z.uuid(),
  minimumPlays: z.number().int().positive(),
});

export type PendingGroupPlay = z.infer<typeof schema>;

/** readGroupPlays recovers this learner's pending gestures while the attempt remains writable. */
export function readGroupPlays(
  studentId: string,
  attemptId: string,
  serverTime: number,
  deadlineAt?: number,
): PendingGroupPlay[] {
  const plays: PendingGroupPlay[] = [];
  try {
    for (const key of Object.keys(localStorage)) {
      if (!key.startsWith(prefix)) continue;
      const parsed = schema.safeParse(parse(localStorage.getItem(key)));
      if (!parsed.success) {
        localStorage.removeItem(key);
        continue;
      }
      const owned =
        parsed.data.studentId === studentId && parsed.data.attemptId === attemptId;
      const expires = owned
        ? (deadlineAt ?? parsed.data.deadlineAt)
        : parsed.data.deadlineAt;
      if (expires <= serverTime) localStorage.removeItem(key);
      else if (owned) plays.push({ ...parsed.data, deadlineAt: expires });
    }
  } catch {
    return plays;
  }
  return plays.sort((a, b) => a.minimumPlays - b.minimumPlays);
}

function parse(raw: string | null): unknown {
  try {
    return raw === null ? null : JSON.parse(raw);
  } catch {
    return null;
  }
}

/** persistGroupPlay keeps a gesture's identity for idempotent retry after a lost response or reload. */
export function persistGroupPlay(play: PendingGroupPlay): void {
  try {
    localStorage.setItem(prefix + play.playId, JSON.stringify(play));
  } catch {
    return;
  }
}

/** forgetGroupPlay removes only an acknowledged gesture, leaving other tabs' writes intact. */
export function forgetGroupPlay(playId: string): void {
  try {
    localStorage.removeItem(prefix + playId);
  } catch {
    return;
  }
}

/** clearGroupPlayDrafts removes unconfirmed listening telemetry from a shared browser on logout. */
export function clearGroupPlayDrafts(): void {
  try {
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith(prefix)) localStorage.removeItem(key);
    }
  } catch {
    return;
  }
}
