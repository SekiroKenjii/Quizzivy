import { describe, expect, it, vi } from "vitest";
import { SheetNoteController } from "@/features/attempts/components/sheetNotes";

function deferred() {
  let resolve!: () => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<void>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

describe("sheet-owned attempt note controller", () => {
  it("changes locally and saves an immutable identity only when flushed", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const notes = new SheetNoteController(save);
    notes.accept("A", "initial");
    notes.change("A", "edited");
    expect(save).not.toHaveBeenCalled();
    expect(notes.unsettled).toBe(true);
    expect(await notes.flush("A")).toBe(true);
    expect(save).toHaveBeenCalledExactlyOnceWith("A", "edited");
    expect(notes.get("A")).toEqual({
      value: "edited",
      saved: "edited",
      pending: false,
      error: null,
    });
    expect(notes.unsettled).toBe(false);
  });

  it("serializes concurrent flushes and drains a newer edit made during the first request", async () => {
    const first = deferred();
    const second = deferred();
    const save = vi
      .fn()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    const notes = new SheetNoteController(save);
    notes.accept("A", null);
    notes.change("A", "first");
    const blur = notes.flush("A");
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    notes.change("A", "latest");
    const close = notes.flushAll();
    expect(save).toHaveBeenCalledTimes(1);
    first.resolve();
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(2));
    expect(save.mock.calls).toEqual([
      ["A", "first"],
      ["A", "latest"],
    ]);
    expect(notes.get("A")).toMatchObject({
      value: "latest",
      saved: "first",
      pending: true,
    });
    second.resolve();
    expect(await blur).toBe(true);
    expect(await close).toBe(true);
    expect(notes.get("A")).toMatchObject({
      value: "latest",
      saved: "latest",
      pending: false,
    });
    expect(notes.unsettled).toBe(false);
  });

  it("does not let an A response overwrite B or newer A text", async () => {
    const pending = deferred();
    const save = vi
      .fn()
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValue(undefined);
    const notes = new SheetNoteController(save);
    notes.accept("A", null);
    notes.accept("B", "B server");
    notes.change("A", "A sent");
    const flushing = notes.flush("A");
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    notes.change("B", "B draft");
    notes.change("A", "A latest");
    notes.accept("A", "stale refetch");
    notes.accept("B", "stale B refetch");
    pending.resolve();
    await flushing;
    expect(notes.get("A")).toMatchObject({ value: "A latest", saved: "A latest" });
    expect(notes.get("B")).toMatchObject({ value: "B draft", saved: "B server" });
    expect(save.mock.calls).toEqual([
      ["A", "A sent"],
      ["A", "A latest"],
    ]);
  });

  it("retains a failed latest draft, stops departure, and retries without an automatic write loop", async () => {
    const pending = deferred();
    const save = vi
      .fn()
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValue(undefined);
    const notes = new SheetNoteController(save);
    notes.accept("A", "saved");
    notes.change("A", "sent");
    const first = notes.flushAll();
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    notes.change("A", "latest");
    const error = new Error("offline");
    pending.reject(error);
    expect(await first).toBe(false);
    expect(notes.get("A")).toMatchObject({
      value: "latest",
      saved: "saved",
      pending: false,
      error,
    });
    notes.accept("A", "refetched");
    expect(notes.get("A")?.value).toBe("latest");
    expect(save).toHaveBeenCalledTimes(1);
    expect(notes.unsettled).toBe(true);
    expect(await notes.flushAll()).toBe(true);
    expect(save.mock.calls[1]).toEqual(["A", "latest"]);
    expect(notes.unsettled).toBe(false);
  });

  it("maps blank text to null and preserves meaningful whitespace", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const notes = new SheetNoteController(save);
    notes.accept("A", "old");
    notes.change("A", " \n\t ");
    await notes.flush("A");
    expect(save).toHaveBeenLastCalledWith("A", null);
    notes.change("A", "  meaningful\n ");
    await notes.flush("A");
    expect(save).toHaveBeenLastCalledWith("A", "  meaningful\n ");
  });

  it("accepts 2000 characters, refuses 2001 without a request, and preserves recovery text", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const notes = new SheetNoteController(save);
    notes.accept("A", null);
    notes.change("A", "x".repeat(2000));
    expect(await notes.flush("A")).toBe(true);
    notes.change("A", "x".repeat(2001));
    expect(await notes.flush("A")).toBe(false);
    expect(save).toHaveBeenCalledTimes(1);
    expect(notes.get("A")?.value).toHaveLength(2001);
    expect(notes.unsettled).toBe(true);
    notes.change("A", "corrected");
    expect(await notes.flush("A")).toBe(true);
  });

  it("explicit discard waits for an in-flight write and restores its actual saved value", async () => {
    const pending = deferred();
    const save = vi.fn().mockReturnValue(pending.promise);
    const notes = new SheetNoteController(save);
    notes.accept("A", "old");
    notes.change("A", "sent");
    const saving = notes.flush("A");
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    notes.change("A", "discard this");
    const discard = notes.discardAll();
    pending.reject(new Error("offline"));
    await discard;
    expect(await saving).toBe(false);
    expect(notes.get("A")).toEqual({
      value: "old",
      saved: "old",
      pending: false,
      error: null,
    });
    expect(notes.unsettled).toBe(false);
  });

  it("a rejected A save does not clear an independent B draft", async () => {
    const pending = deferred();
    const notes = new SheetNoteController(() => pending.promise);
    notes.accept("A", null);
    notes.accept("B", null);
    notes.change("A", "A draft");
    notes.change("B", "B draft");
    const saving = notes.flush("A");
    pending.reject(new Error("refused"));
    expect(await saving).toBe(false);
    expect(notes.get("B")).toEqual({
      value: "B draft",
      saved: null,
      pending: false,
      error: null,
    });
  });
  it("adopts a new server note only while its existing draft is settled", () => {
    const notes = new SheetNoteController(vi.fn().mockResolvedValue(undefined));
    notes.accept("A", "original");
    notes.accept("A", "co-teacher edit");
    expect(notes.get("A")).toMatchObject({
      value: "co-teacher edit",
      saved: "co-teacher edit",
    });
    notes.change("A", "my draft");
    notes.accept("A", "another server edit");
    expect(notes.get("A")).toMatchObject({
      value: "my draft",
      saved: "co-teacher edit",
    });
  });

  it("waits for all active writes before discard and restores the actual last saved value", async () => {
    const first = deferred();
    const second = deferred();
    const save = vi
      .fn()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    const notes = new SheetNoteController(save);
    notes.accept("A", "original");
    notes.change("A", "first");
    const flushing = notes.flush("A");
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    notes.change("A", "second");
    const discard = notes.discardAll();
    first.resolve();
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(2));
    second.resolve();
    await flushing;
    await discard;
    expect(notes.unsettled).toBe(false);
    expect(notes.get("A")).toMatchObject({
      value: "second",
      saved: "second",
      pending: false,
    });
  });
});
