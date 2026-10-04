import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const A = "018f0000-0000-7000-8000-00000000a001";
const B = "018f0000-0000-7000-8000-00000000b002";
const KEY = "quizzivy.session.att-1";

async function fresh() {
  vi.resetModules();
  return import("@/features/take-test/heldSession");
}

beforeEach(() => {
  sessionStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  sessionStorage.clear();
});

describe("the session a tab holds for an attempt", () => {
  it("holds nothing for an attempt it was never given", async () => {
    const { heldSession } = await fresh();

    expect(heldSession("att-1")).toBeNull();
  });

  it("names the session it was given, and keeps it in the tab's storage", async () => {
    const { heldSession, holdSession } = await fresh();

    holdSession("att-1", A);

    expect(heldSession("att-1")).toBe(A);
    expect(sessionStorage.getItem(KEY)).toBe(A);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("names it again after a reload", async () => {
    sessionStorage.setItem(KEY, A);

    const { heldSession } = await fresh();

    expect(heldSession("att-1")).toBe(A);
  });

  it("holds one session per attempt", async () => {
    const { heldSession, holdSession } = await fresh();

    holdSession("att-1", A);
    holdSession("att-2", B);

    expect(heldSession("att-1")).toBe(A);
    expect(heldSession("att-2")).toBe(B);
    expect(sessionStorage.getItem(KEY)).toBe(A);
    expect(sessionStorage.getItem("quizzivy.session.att-2")).toBe(B);

    const reloaded = await fresh();
    expect(reloaded.heldSession("att-1")).toBe(A);
    expect(reloaded.heldSession("att-2")).toBe(B);
  });

  it("names nothing when the stored value is not a session id", async () => {
    sessionStorage.setItem(KEY, "<script>");

    const { heldSession } = await fresh();

    expect(heldSession("att-1")).toBeNull();
  });

  it("removes the stored session when the storage refuses the new one", async () => {
    sessionStorage.setItem(KEY, A);
    const { heldSession, holdSession } = await fresh();
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });

    holdSession("att-1", B);

    expect(sessionStorage.getItem(KEY)).toBeNull();
    expect(heldSession("att-1")).toBe(B);

    const reloaded = await fresh();
    expect(reloaded.heldSession("att-1")).toBeNull();
  });

  it("names nothing when the storage cannot be read", async () => {
    sessionStorage.setItem(KEY, A);
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });

    const { heldSession } = await fresh();

    expect(heldSession("att-1")).toBeNull();
  });
});
