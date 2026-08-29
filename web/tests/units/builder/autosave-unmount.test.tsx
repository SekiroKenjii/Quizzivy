import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import {
  mergeAutosave,
  useAutosave,
  type AutosaveStatus,
} from "@/features/tests/useAutosave";

/**
 * The edit made inside the debounce window has to survive the component going
 * away.
 *
 * In the builder, "type, then click the next question" swaps the editor within
 * a second — the ordinary way to work. Dropping the timer without saving loses
 * that edit silently, while the indicator still reports the last save that did
 * land.
 */
function Harness({
  save,
  onReady,
}: {
  save: (value: string) => Promise<void>;
  onReady: (schedule: (value: string) => void) => void;
}) {
  const autosave = useAutosave<string>({ save });

  // Scheduling from an effect rather than during render: reporting "saved"
  // re-renders, and a render-time schedule would re-queue on that render —
  // a property of the harness, not of the hook.
  useEffect(() => {
    onReady(autosave.schedule);
  }, [onReady, autosave.schedule]);

  return null;
}

let saved: string[] = [];

function save(value: string) {
  saved.push(value);
  return Promise.resolve();
}

function mount() {
  let schedule: (value: string) => void = () => {
    throw new Error("schedule was never handed over");
  };
  const view = render(<Harness save={save} onReady={(s) => (schedule = s)} />);
  return { view, schedule: (value: string) => act(() => schedule(value)) };
}

beforeEach(() => {
  saved = [];
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("autosave and unmounting", () => {
  it("saves a pending edit when the editor is swapped out", () => {
    const { view, schedule } = mount();

    schedule("nửa câu");
    expect(saved, "nothing should have been sent yet").toEqual([]);

    view.unmount();

    expect(saved).toEqual(["nửa câu"]);
  });

  it("does not save again when nothing is pending", () => {
    const { view, schedule } = mount();

    schedule("xong");
    act(() => {
      vi.advanceTimersByTime(1500);
    });
    expect(saved).toEqual(["xong"]);

    view.unmount();

    expect(saved, "the debounced save already ran").toEqual(["xong"]);
  });

  it("sends only the last value when several edits land in the window", () => {
    const { view, schedule } = mount();

    schedule("a");
    schedule("ab");
    schedule("abc");
    view.unmount();

    expect(saved).toEqual(["abc"]);
  });
});

describe("merging the builder's two autosave statuses", () => {
  const older: AutosaveStatus = { kind: "saved", at: new Date("2026-01-01T10:00:00Z") };
  const newer: AutosaveStatus = { kind: "saved", at: new Date("2026-01-01T10:05:00Z") };

  it("reports the most recent save, not whichever came first", () => {
    expect(mergeAutosave([older, newer])).toBe(newer);
    expect(mergeAutosave([newer, older])).toBe(newer);
  });

  it("lets a failure outrank a success, whichever side it is on", () => {
    const failed: AutosaveStatus = { kind: "failed", message: "" };
    expect(mergeAutosave([newer, failed])).toBe(failed);
    expect(mergeAutosave([failed, newer])).toBe(failed);
  });

  it("puts stale above everything, since nothing more will be saved", () => {
    const stale: AutosaveStatus = { kind: "stale" };
    expect(mergeAutosave([stale, { kind: "failed", message: "" }, newer])).toBe(stale);
  });

  it("says saving while anything is in flight", () => {
    expect(mergeAutosave([newer, { kind: "saving" }])).toEqual({ kind: "saving" });
  });

  it("says nothing when nothing has happened", () => {
    expect(mergeAutosave([{ kind: "idle" }, { kind: "idle" }])).toEqual({
      kind: "idle",
    });
  });
});
