import { beforeEach, describe, expect, it } from "vitest";
import { rememberPending, takePending } from "@/features/auth/google/pkce";

// takePending is the door through which sessionStorage enters the OAuth
// callback, and every field it returns is load-bearing: `verifier` goes to
// Google's token endpoint, `state` is what statesMatch compares against the URL
// parameter, `mode` decides whether the returning code is redeemed at
// /auth/google or /auth/google/link, and `next` has string methods called on it.
//
// The whole module was untested when its type guard was added.

// Discovered rather than hardcoded. An earlier draft of this file guessed the
// key wrong, and every "returns null" case passed for that reason instead of
// because the guard rejected anything.
function storageKey(): string {
  sessionStorage.clear();
  rememberPending({ verifier: "x", state: "y", mode: "signin" });
  const key = sessionStorage.key(0);
  if (!key) throw new Error("rememberPending stored nothing");
  sessionStorage.clear();
  return key;
}

const KEY = storageKey();

function store(value: unknown) {
  sessionStorage.setItem(KEY, JSON.stringify(value));
}

const valid = {
  verifier: "a".repeat(43),
  state: "b".repeat(43),
  mode: "signin" as const,
};

beforeEach(() => {
  sessionStorage.clear();
});

describe("takePending", () => {
  it("round-trips a well-formed record", () => {
    rememberPending({ ...valid, next: "/admin", joinCode: "K7M3P9QR" });

    const got = takePending();
    expect(got).not.toBeNull();
    expect(got?.verifier).toBe(valid.verifier);
    expect(got?.state).toBe(valid.state);
    expect(got?.mode).toBe("signin");
    expect(got?.next).toBe("/admin");
    expect(got?.joinCode).toBe("K7M3P9QR");
  });

  it("clears the entry, so a verifier is single-use", () => {
    rememberPending(valid);

    expect(takePending()).not.toBeNull();
    expect(takePending()).toBeNull();
    expect(sessionStorage.getItem(KEY)).toBeNull();
  });

  it("returns null when nothing is stored", () => {
    expect(takePending()).toBeNull();
  });

  it("returns null for a malformed entry rather than throwing", () => {
    sessionStorage.setItem(KEY, "{not json");
    expect(takePending()).toBeNull();
  });

  it.each([
    ["missing verifier", { state: valid.state, mode: "signin" }],
    ["missing state", { verifier: valid.verifier, mode: "signin" }],
    ["missing mode", { verifier: valid.verifier, state: valid.state }],
    ["a non-string verifier", { ...valid, verifier: 42 }],
    ["a null instead of a record", null],
    ["an array", [valid]],
    ["a bare string", "nope"],
  ])("returns null for %s", (_label, value) => {
    store(value);
    expect(takePending()).toBeNull();
  });

  // The case that decides which endpoint the code is sent to. Exchanging a link
  // request at the sign-in endpoint would replace the current session with
  // whichever Google account was chosen.
  it("returns null for an unrecognised mode", () => {
    store({ ...valid, mode: "nonsense" });
    expect(takePending()).toBeNull();
  });

  it("accepts both real modes", () => {
    for (const mode of ["signin", "link"] as const) {
      store({ ...valid, mode });
      expect(takePending()?.mode).toBe(mode);
    }
  });

  // `next` reaches destinationAfterSignIn, which calls .startsWith on it. A
  // truthy non-string is the shape that throws inside the callback effect.
  it.each([
    ["an object next", { ...valid, next: {} }],
    ["a numeric next", { ...valid, next: 7 }],
    ["an object joinCode", { ...valid, joinCode: {} }],
  ])("returns null for %s", (_label, value) => {
    store(value);
    expect(takePending()).toBeNull();
  });

  it("accepts a record with the optional fields absent", () => {
    store(valid);
    const got = takePending();
    expect(got).not.toBeNull();
    expect(got?.next).toBeUndefined();
    expect(got?.joinCode).toBeUndefined();
  });
});
