import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { clearSession, pending } from "@/features/integrity/buffer";

afterEach(() => clearSession("context-attempt"));

it("captures the question at event time without rebinding the away episode", () => {
  const { rerender } = renderHook(
    ({ questionId }: { questionId: string | null }) =>
      useIntegrityMonitor({
        attemptId: "context-attempt",
        sessionId: "session-1",
        beaconToken: "",
        policy: null,
        questionId,
      }),
    { initialProps: { questionId: "q1" } as { questionId: string | null } },
  );
  act(() => window.dispatchEvent(new Event("blur")));
  rerender({ questionId: "q2" });
  act(() => window.dispatchEvent(new Event("focus")));
  rerender({ questionId: null });
  act(() => window.dispatchEvent(new Event("offline")));
  const events = pending();
  expect(events[0]).toMatchObject({ kind: "window_blur", questionId: "q1" });
  expect(events[1]).toMatchObject({
    kind: "window_focus",
    questionId: "q2",
    meta: { awayMs: expect.any(Number) },
  });
  expect(events[2]).not.toHaveProperty("questionId");
});
