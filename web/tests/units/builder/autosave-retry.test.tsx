import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { useAutosave, type AutosaveStatus } from "@/features/tests/useAutosave";
import { ApiError } from "@/lib/api/errors";

interface Handle {
  schedule: (value: string) => void;
  retry: () => void;
  status: AutosaveStatus;
}

function Harness({
  save,
  onReady,
}: {
  save: (value: string) => Promise<void>;
  onReady: (handle: Handle) => void;
}) {
  const { schedule, retry, status } = useAutosave<string>({ save });
  useEffect(() => {
    onReady({ schedule, retry, status });
  }, [onReady, schedule, retry, status]);
  return null;
}

let saved: string[] = [];
let failNext = false;

function save(value: string) {
  saved.push(value);
  if (failNext) {
    failNext = false;
    return Promise.reject(
      new ApiError({
        status: 400,
        code: "VALIDATION_FAILED",
        message: "Tên đề quá dài.",
      }),
    );
  }
  return Promise.resolve();
}

function mount() {
  let handle: Handle = {
    schedule: () => {
      throw new Error("not ready");
    },
    retry: () => {
      throw new Error("not ready");
    },
    status: { kind: "idle" },
  };
  render(<Harness save={save} onReady={(h) => (handle = h)} />);
  return () => handle;
}

beforeEach(() => {
  saved = [];
  failNext = false;
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("a failed autosave", () => {
  it("keeps the server's reason and sends the same value again on retry", async () => {
    const current = mount();
    failNext = true;
    act(() => current().schedule("Unit 5 — a very long title"));
    await act(async () => {
      await vi.runAllTimersAsync();
    });

    const failed = current().status;
    expect(failed.kind).toBe("failed");
    expect(failed.kind === "failed" && failed.message).toBe("Tên đề quá dài.");
    expect(saved).toEqual(["Unit 5 — a very long title"]);

    await act(async () => {
      current().retry();
      await vi.runAllTimersAsync();
    });
    expect(saved).toEqual(["Unit 5 — a very long title", "Unit 5 — a very long title"]);
    expect(current().status.kind).toBe("saved");
  });
});
