import { expect, it, vi } from "vitest";

vi.mock("@/features/take-test/store", () => {
  throw new Error("preview must not import the store");
});
vi.mock("@/features/take-test/livePaper", () => {
  throw new Error("preview must not import the live paper source");
});
vi.mock("@/features/integrity/useIntegrityMonitor", () => {
  throw new Error("preview must not import the integrity monitor");
});

it("loads the engine's preview with no store, live source or integrity monitor in its imports", async () => {
  const engine = await import("@/features/take-test");
  expect(engine.EnginePreview).toBeTypeOf("function");
});
