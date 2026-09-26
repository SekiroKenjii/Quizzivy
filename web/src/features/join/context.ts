/** Join context: the class a visitor chose before signing in, and what came of it. */

/** JoinContext is the class a signed-out visitor chose to join, kept across sign-in. */
export type JoinContext = {
  code: string;
  className: string;
  teacherName: string;
};

/**
 * JoinOutcome is what `/join/:code` shows once an enrolment has been tried. It
 * travels in the router's location state, never in the URL.
 */
export type JoinOutcome =
  | { kind: "joined"; className: string; teacherName: string }
  | { kind: "failed"; className: string; message: string };

const STORAGE_KEY = "quizzivy.join";
const LIFETIME_MS = 30 * 60 * 1000;

/** saveJoinContext remembers the chosen class for thirty minutes in this tab. */
export function saveJoinContext(context: JoinContext): void {
  try {
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ ...context, savedAt: Date.now() }),
    );
  } catch {
    return;
  }
}

/**
 * readJoinContext returns the remembered class, or null when there is none,
 * it is older than thirty minutes, or storage is unavailable or tampered with.
 */
export function readJoinContext(): JoinContext | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (isStored(parsed) && Date.now() - parsed.savedAt < LIFETIME_MS) {
      return {
        code: parsed.code,
        className: parsed.className,
        teacherName: parsed.teacherName,
      };
    }
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    return null;
  }
  return null;
}

/** clearJoinContext forgets the remembered class. */
export function clearJoinContext(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    return;
  }
}

/** joinOutcomeState wraps an outcome as the location state `/join/:code` reads. */
export function joinOutcomeState(outcome: JoinOutcome): { joinOutcome: JoinOutcome } {
  return { joinOutcome: outcome };
}

/** readJoinOutcome returns the outcome carried in a location state, if it holds a valid one. */
export function readJoinOutcome(state: unknown): JoinOutcome | null {
  if (typeof state !== "object" || state === null) return null;
  const outcome = (state as Record<string, unknown>)["joinOutcome"];
  if (typeof outcome !== "object" || outcome === null) return null;
  const o = outcome as Record<string, unknown>;
  if (typeof o["className"] !== "string") return null;
  if (o["kind"] === "joined" && typeof o["teacherName"] === "string") {
    return { kind: "joined", className: o["className"], teacherName: o["teacherName"] };
  }
  if (o["kind"] === "failed" && typeof o["message"] === "string") {
    return { kind: "failed", className: o["className"], message: o["message"] };
  }
  return null;
}

function isStored(value: unknown): value is JoinContext & { savedAt: number } {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v["code"] === "string" &&
    typeof v["className"] === "string" &&
    typeof v["teacherName"] === "string" &&
    typeof v["savedAt"] === "number"
  );
}
