/** CookieKind names the four cookie-mutating authentication operations. */
export type CookieKind = "refresh" | "login" | "google" | "logout";

/** AccountTransition reserves one explicit account change until its raw work drains. */
export interface AccountTransition {
  readonly kind: Exclude<CookieKind, "refresh">;
  readonly generation: number;
  interested: boolean;
  phase: "account" | "cleanup";
}

/** TransitionPendingError reports incomplete work without releasing transport ownership. */
export class TransitionPendingError extends Error {
  readonly phase: "account" | "cleanup";
  constructor(phase: "account" | "cleanup") {
    super(phase);
    this.phase = phase;
    this.name = "TransitionPendingError";
  }
}

interface CookieOwner {
  kind: CookieKind;
  generation: number;
  completion: Promise<unknown>;
}

let active: CookieOwner | null = null;
let admission: AccountTransition | null = null;
const listeners = new Set<() => void>();
let status: {
  kind: CookieKind | "idle";
  cookieKind: CookieKind | null;
  phase: "account" | "cleanup";
} = { kind: "idle", cookieKind: null, phase: "account" };

function publish() {
  status = {
    kind: admission?.kind ?? active?.kind ?? "idle",
    cookieKind: active?.kind ?? null,
    phase: admission?.phase ?? "account",
  };
  for (const listener of listeners) listener();
}

/** transitionStatus reads the current admission and raw transport status. */
export function transitionStatus() {
  return status;
}

/** subscribeTransitions observes admission changes without starting or replaying work. */
export function subscribeTransitions(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** reserveTransition synchronously admits at most one explicit account intention. */
export function reserveTransition(
  kind: AccountTransition["kind"],
  generation: number,
): AccountTransition {
  if (admission) throw new TransitionPendingError(admission.phase);
  const ticket: AccountTransition = {
    kind,
    generation,
    interested: true,
    phase: "account",
  };
  admission = ticket;
  publish();
  return ticket;
}

/** isTransitionCurrent checks both reservation identity and the caller's continuing interest. */
export function isTransitionCurrent(ticket: AccountTransition) {
  return admission === ticket && ticket.interested;
}

/** cancelTransitionInterest discards late UI outcomes while retaining admission ownership. */
export function cancelTransitionInterest(ticket: AccountTransition) {
  ticket.interested = false;
}

/** setTransitionPhase exposes cleanup status while retaining the same account owner. */
export function setTransitionPhase(
  ticket: AccountTransition,
  phase: AccountTransition["phase"],
) {
  if (admission !== ticket) return;
  ticket.phase = phase;
  publish();
}

/** finishTransition releases only the matching reservation after its work has actually settled. */
export function finishTransition(ticket: AccountTransition) {
  if (admission !== ticket) return;
  admission = null;
  publish();
}

/** cookieOperation holds raw ownership through the callback's complete response handling. */
export async function cookieOperation<T>(
  kind: CookieKind,
  generation: number,
  work: () => Promise<T>,
  ticket?: AccountTransition,
): Promise<T> {
  const local =
    kind !== "refresh" && !ticket ? reserveTransition(kind, generation) : null;
  const reservation = ticket ?? local;
  if (kind === "refresh" && admission)
    throw new TransitionPendingError(admission.phase);
  if (reservation && admission !== reservation)
    throw new TransitionPendingError("account");
  try {
    while (active) await active.completion.catch(() => undefined);
    if (reservation && (admission !== reservation || !reservation.interested))
      throw new DOMException("Superseded", "AbortError");
    const mine: CookieOwner = { kind, generation, completion: Promise.resolve() };
    active = mine;
    const completion = Promise.resolve()
      .then(work)
      .finally(() => {
        if (active === mine) {
          active = null;
          publish();
        }
      });
    mine.completion = completion;
    publish();
    return await completion;
  } finally {
    if (local) finishTransition(local);
  }
}

/** waitForTransition bounds UI waiting without canceling or forgetting underlying work. */
export async function waitForTransition<T>(
  ticket: AccountTransition,
  completion: Promise<T>,
): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      completion,
      new Promise<never>((_, reject) => {
        timer = setTimeout(
          () => {
            ticket.interested = false;
            reject(new TransitionPendingError(ticket.phase));
          },
          ticket.phase === "cleanup" ? 10_000 : 20_000,
        );
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}
