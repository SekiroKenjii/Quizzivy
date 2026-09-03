/** PKCE for the Google authorization-code flow (§5.3, O-13). */

const AUTHORIZATION_ENDPOINT = "https://accounts.google.com/o/oauth2/v2/auth";

/** Long enough to be unguessable, inside the contract's 43..128. */
const VERIFIER_BYTES = 48;

const STORAGE_KEY = "quizzivy.oauth.pending";

export interface PendingAuthorization {
  verifier: string;
  state: string;
  // What the returning code is FOR.
  mode: "signin" | "link";
  /** Where to go after a successful sign-in. Carried across the round trip. */
  next?: string;
  /** Set when the flow started from /join, so the callback can enrol. */
  joinCode?: string;
}

/** The pending authorization survives in `sessionStorage`. */
export function rememberPending(pending: PendingAuthorization) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(pending));
  } catch {
    // Private mode or a full quota: the sign-in fails later with a clear error.
  }
}

/** Reads and CLEARS the pending authorization. Single-use, by construction. */
export function takePending(): PendingAuthorization | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    sessionStorage.removeItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    return isPendingAuthorization(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

/** Validates what came out of storage before any of it is trusted. */
function isPendingAuthorization(value: unknown): value is PendingAuthorization {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v["state"] === "string" &&
    typeof v["verifier"] === "string" &&
    (v["mode"] === "signin" || v["mode"] === "link") &&
    isAbsentOrString(v["next"]) &&
    isAbsentOrString(v["joinCode"])
  );
}

function isAbsentOrString(value: unknown): boolean {
  return value === undefined || typeof value === "string";
}

function base64UrlEncode(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function randomUrlSafe(bytes: number): string {
  const raw = new Uint8Array(bytes);
  crypto.getRandomValues(raw);
  return base64UrlEncode(raw);
}

async function s256(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(verifier),
  );
  return base64UrlEncode(new Uint8Array(digest));
}

/** `state` is compared in constant time on the way back. */
export function statesMatch(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}

/** The redirect Google sends the browser back to. Allow-listed server-side. */
export function callbackUrl(): string {
  return `${window.location.origin}/auth/google/callback`;
}

export interface AuthorizationRequest {
  url: string;
  pending: PendingAuthorization;
}

/** Builds the authorization request and the state that has to outlive it. */
export async function buildAuthorizationRequest(options: {
  clientId: string;
  mode?: "signin" | "link";
  next?: string | undefined;
  joinCode?: string | undefined;
}): Promise<AuthorizationRequest> {
  const verifier = randomUrlSafe(VERIFIER_BYTES);
  const state = randomUrlSafe(16);

  const params = new URLSearchParams({
    client_id: options.clientId,
    redirect_uri: callbackUrl(),
    response_type: "code",
    scope: "openid email profile",
    code_challenge: await s256(verifier),
    code_challenge_method: "S256",
    state,
    prompt: "select_account",
  });

  return {
    url: `${AUTHORIZATION_ENDPOINT}?${params.toString()}`,
    pending: {
      verifier,
      state,
      mode: options.mode ?? "signin",
      ...(options.next ? { next: options.next } : {}),
      ...(options.joinCode ? { joinCode: options.joinCode } : {}),
    },
  };
}
