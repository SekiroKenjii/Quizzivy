import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { AutosaveStatusLabel } from "@/features/tests/components/AutosaveStatusLabel";
import { mergeAutosave, useAutosave } from "@/features/tests/useAutosave";
import "@/lib/i18n";

/** §8's badge answers one question: is it safe to close the tab. */
function Harness({
  onReady,
}: Readonly<{ onReady: (schedule: (v: string) => void) => void }>) {
  const autosave = useAutosave<string>({ save: () => Promise.resolve() });
  useEffect(() => {
    onReady(autosave.schedule);
  }, [onReady, autosave.schedule]);
  return <AutosaveStatusLabel status={autosave.status} />;
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the autosave badge during the debounce window", () => {
  it("says not saved the moment an edit is scheduled", () => {
    let schedule: (v: string) => void = (_value) => undefined;
    render(<Harness onReady={(s) => (schedule = s)} />);

    act(() => schedule("một chữ"));

    expect(screen.getByText("Chưa lưu")).toBeInTheDocument();
    expect(screen.queryByText(/Đã lưu/)).toBeNull();
  });

  it("only claims saved once the request has come back", async () => {
    let schedule: (v: string) => void = (_value) => undefined;
    render(<Harness onReady={(s) => (schedule = s)} />);

    act(() => schedule("một chữ"));
    await act(async () => {
      vi.advanceTimersByTime(1500);
      await Promise.resolve();
    });

    expect(screen.getByText(/Đã lưu \d\d:\d\d/)).toBeInTheDocument();
    expect(screen.queryByText("Chưa lưu")).toBeNull();
  });

  it("goes back to not-saved when the teacher types again", async () => {
    let schedule: (v: string) => void = (_value) => undefined;
    render(<Harness onReady={(s) => (schedule = s)} />);

    act(() => schedule("một"));
    await act(async () => {
      vi.advanceTimersByTime(1500);
      await Promise.resolve();
    });
    expect(screen.getByText(/Đã lưu/)).toBeInTheDocument();

    act(() => schedule("một hai"));

    expect(screen.getByText("Chưa lưu")).toBeInTheDocument();
  });
});

describe("merging a dirty autosave with a saved one", () => {
  const saved = { kind: "saved" as const, at: new Date("2026-01-01T10:00:00Z") };

  it("reports not-saved: one unsent edit makes the whole screen unsaved", () => {
    expect(mergeAutosave([saved, { kind: "dirty" }])).toEqual({ kind: "dirty" });
    expect(mergeAutosave([{ kind: "dirty" }, saved])).toEqual({ kind: "dirty" });
  });

  it("prefers saving, which is the more informative in-progress answer", () => {
    expect(mergeAutosave([{ kind: "dirty" }, { kind: "saving" }])).toEqual({
      kind: "saving",
    });
  });

  it("still puts stale and failed above it", () => {
    expect(mergeAutosave([{ kind: "dirty" }, { kind: "stale" }])).toEqual({
      kind: "stale",
    });
    expect(mergeAutosave([{ kind: "dirty" }, { kind: "failed", message: "" }])).toEqual(
      { kind: "failed", message: "" },
    );
  });
});
