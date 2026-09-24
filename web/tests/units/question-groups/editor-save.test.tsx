import { act, renderHook } from "@testing-library/react";
import { useGroupEditor } from "@/features/question-groups/useGroupEditor";
import { emptyGroup } from "@/features/question-groups/model";
import type { GroupBundle, StoredGroup } from "@/features/question-groups/api";
import type { DraftScope } from "@/lib/drafts/store";
import { ApiError } from "@/lib/api/errors";
import "@/lib/i18n";

function stored(): StoredGroup {
  return {
    bundle: emptyGroup("Passage"),
    ownerSectionId: null,
    revision: 1,
    archivedAt: null,
    createdAt: "2026-09-24T00:00:00Z",
    updatedAt: "2026-09-24T00:00:00Z",
    assets: [],
    unavailableAssetIds: [],
  };
}
function scope(): DraftScope {
  return {
    read: vi.fn().mockResolvedValue(null),
    write: vi.fn().mockResolvedValue(undefined),
    remove: vi.fn().mockResolvedValue(undefined),
  };
}
beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

function renamed(bundle: GroupBundle, title: string): GroupBundle {
  return { ...bundle, group: { ...bundle.group, title } };
}

test("a save acknowledgement never discards newer edits and advances their local base revision", async () => {
  const initial = stored();
  const local = scope();
  let resolve: (value: StoredGroup) => void = (_value: StoredGroup) => undefined;
  const save = vi
    .fn()
    .mockImplementationOnce(
      () =>
        new Promise<StoredGroup>((done) => {
          resolve = done;
        }),
    )
    .mockImplementation(async (bundle: GroupBundle, revision: number) => ({
      ...initial,
      bundle,
      revision: revision + 1,
    }));
  const { result } = renderHook(() =>
    useGroupEditor({ stored: initial, recovered: null, scope: local, save }),
  );
  const first = renamed(initial.bundle, "First");
  act(() => result.current.change(first));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1500);
  });
  expect(save).toHaveBeenCalledWith(first, 1, undefined);
  const second = renamed(initial.bundle, "Second");
  act(() => result.current.change(second));
  await act(async () => {
    resolve({ ...initial, bundle: first, revision: 2 });
  });
  expect(result.current.bundle.group.title).toBe("Second");
  expect(result.current.dirty).toBe(true);
  expect(local.remove).not.toHaveBeenCalled();
  expect(local.write).toHaveBeenLastCalledWith(
    expect.any(String),
    expect.any(Number),
    expect.objectContaining({ revision: 2, bundle: second }),
  );
  await act(async () => {
    await result.current.saveNow();
  });
  expect(save).toHaveBeenLastCalledWith(second, 2, undefined);
  expect(result.current.dirty).toBe(false);
  expect(local.remove).toHaveBeenCalled();
});

test("failed and incomplete writes remain recoverable, while retry saves the latest edit", async () => {
  const initial = stored();
  const local = scope();
  const save = vi
    .fn()
    .mockRejectedValueOnce(new Error("offline"))
    .mockImplementation(async (bundle: GroupBundle) => ({
      ...initial,
      bundle,
      revision: 2,
    }));
  const { result } = renderHook(() =>
    useGroupEditor({ stored: initial, recovered: null, scope: local, save }),
  );
  act(() => result.current.change(renamed(initial.bundle, "Changed")));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1500);
  });
  expect(result.current.status.kind).toBe("failed");
  expect(result.current.local).toBe("stored");
  expect(result.current.dirty).toBe(true);
  await act(async () => {
    await result.current.saveNow();
  });
  expect(result.current.dirty).toBe(false);
  act(() => result.current.change(renamed(initial.bundle, "")));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1500);
  });
  expect(save).toHaveBeenCalledTimes(2);
  expect(result.current.local).toBe("stored");
  expect(result.current.status.kind).toBe("failed");
});

test("stale or recovered conflicting revisions keep local edits and never overwrite the server", async () => {
  const initial = stored();
  const local = scope();
  const save = vi
    .fn()
    .mockRejectedValue(
      new ApiError({ status: 409, code: "STALE_WRITE", message: "stale" }),
    );
  const { result } = renderHook(() =>
    useGroupEditor({ stored: initial, recovered: null, scope: local, save }),
  );
  act(() => result.current.change(renamed(initial.bundle, "My change")));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1500);
  });
  expect(result.current.copyRequired).toBe(true);
  act(() => result.current.change(renamed(initial.bundle, "Recovered further")));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1500);
  });
  expect(save).toHaveBeenCalledTimes(1);
  expect(local.write).toHaveBeenLastCalledWith(
    expect.any(String),
    expect.any(Number),
    expect.objectContaining({
      bundle: expect.objectContaining({
        group: expect.objectContaining({ title: "Recovered further" }),
      }),
    }),
  );
});

test("unmount preserves edits and storage failure is never shown as a local acknowledgement", async () => {
  const initial = stored();
  const local = scope();
  local.write = vi.fn().mockRejectedValue(new Error("quota"));
  const save = vi.fn().mockRejectedValue(new Error("offline"));
  const { result, unmount } = renderHook(() =>
    useGroupEditor({ stored: initial, recovered: null, scope: local, save }),
  );
  act(() => result.current.change(renamed(initial.bundle, "Unsent")));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(300);
  });
  expect(result.current.local).toBe("unavailable");
  await act(async () => {
    unmount();
  });
  expect(save).toHaveBeenCalledTimes(1);
  expect(local.remove).not.toHaveBeenCalled();
});
