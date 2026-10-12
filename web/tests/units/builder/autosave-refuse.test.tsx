import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import {
  AutosaveRefused,
  useAutosave,
  type AutosaveStatus,
} from "@/features/tests/useAutosave";

interface Handle {
  schedule: (value: string) => void;
  flush: () => Promise<void>;
  status: AutosaveStatus;
}

function refuse(value: string) {
  return value.trim() === "" ? "Lựa chọn không được để trống." : null;
}

function Harness({
  save,
  onReady,
}: {
  save: (value: string) => Promise<void>;
  onReady: (handle: Handle) => void;
}) {
  const { schedule, flush, status } = useAutosave<string>({ save, refuse });
  useEffect(() => {
    onReady({ schedule, flush, status });
  }, [onReady, schedule, flush, status]);
  return null;
}

let saved: string[] = [];

function save(value: string) {
  saved.push(value);
  return Promise.resolve();
}

function mount() {
  let handle: Handle = {
    schedule: () => {
      throw new Error("not ready");
    },
    flush: () => Promise.reject(new Error("not ready")),
    status: { kind: "idle" },
  };
  render(<Harness save={save} onReady={(next) => (handle = next)} />);
  return () => handle;
}

beforeEach(() => {
  saved = [];
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("a value the caller refuses", () => {
  it("is not sent, says why, and is sent once it passes", async () => {
    const current = mount();

    act(() => current().schedule(" "));
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    expect(saved).toEqual([]);
    expect(current().status).toEqual({
      kind: "failed",
      message: "Lựa chọn không được để trống.",
      refused: true,
    });

    act(() => current().schedule("have gone"));
    await act(async () => {
      await vi.runAllTimersAsync();
    });
    expect(saved).toEqual(["have gone"]);
    expect(current().status.kind).toBe("saved");
  });

  it("makes a flush throw, so nothing goes on as if it were saved", async () => {
    const current = mount();
    act(() => current().schedule(""));

    let thrown: unknown;
    await act(async () => {
      await current()
        .flush()
        .catch((cause: unknown) => {
          thrown = cause;
        });
    });
    expect(thrown).toBeInstanceOf(AutosaveRefused);
    expect(saved).toEqual([]);
  });
});
