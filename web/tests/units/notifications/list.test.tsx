import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { http } from "msw";
import { server } from "@tests/support/server";
import { useNotificationList } from "@/features/notifications/useNotificationList";
import {
  BASE,
  harness,
  id,
  page,
  notices,
  listResponse,
  deferred,
  errorResponse,
} from "./support";

describe("notification cursor list", () => {
  it("loads20 then appends the next cursor page, stops at null, and coalesces repeated loadMore", async () => {
    const urls: URL[] = [];
    const gate = deferred<void>();
    server.use(
      http.get(`${BASE}/me/notifications`, async ({ request }) => {
        const url = new URL(request.url);
        urls.push(url);
        if (url.searchParams.has("before")) {
          await gate.promise;
          return listResponse({
            items: [{ ...notices[0]!, id: id(21) }],
            nextBefore: null,
          });
        }
        return listResponse();
      }),
    );
    const hook = renderHook(() => useNotificationList(), harness());
    expect(hook.result.current.isPending).toBe(true);
    await waitFor(() => expect(hook.result.current.items).toHaveLength(20));
    expect(hook.result.current.hasMore).toBe(true);
    expect(urls[0]!.searchParams.get("limit")).toBe("20");
    expect(urls[0]!.searchParams.has("before")).toBe(false);
    act(() => {
      hook.result.current.loadMore();
      hook.result.current.loadMore();
    });
    await waitFor(() => expect(urls).toHaveLength(2));
    expect(urls[1]!.searchParams.get("before")).toBe(page.nextBefore);
    expect(hook.result.current.loadingMore).toBe(true);
    gate.resolve();
    await waitFor(() => expect(hook.result.current.items).toHaveLength(21));
    expect(hook.result.current.hasMore).toBe(false);
    expect(hook.result.current.loadingMore).toBe(false);
    act(() => hook.result.current.loadMore());
    expect(urls).toHaveLength(2);
    expect(hook.result.current.items.at(-1)!.id).toBe(id(21));
  });
  it("refuses a cursor already used even when the server repeats it", async () => {
    const cursors: (string | null)[] = [];
    server.use(
      http.get(`${BASE}/me/notifications`, ({ request }) => {
        const before = new URL(request.url).searchParams.get("before");
        cursors.push(before);
        return listResponse(before ? { items: [], nextBefore: page.nextBefore } : page);
      }),
    );
    const hook = renderHook(() => useNotificationList(), harness());
    await waitFor(() => expect(hook.result.current.hasMore).toBe(true));
    act(() => hook.result.current.loadMore());
    await waitFor(() => expect(hook.result.current.hasMore).toBe(false));
    act(() => hook.result.current.loadMore());
    expect(cursors).toEqual([null, page.nextBefore]);
  });
  it("sends no requests while disabled including an explicit loadMore and exposes failure/refetch", async () => {
    let requests = 0;
    server.use(
      http.get(`${BASE}/me/notifications`, () => {
        requests += 1;
        return errorResponse();
      }),
    );
    const hook = renderHook(({ enabled }) => useNotificationList(enabled), {
      ...harness(),
      initialProps: { enabled: false },
    });
    act(() => hook.result.current.loadMore());
    expect(requests).toBe(0);
    hook.rerender({ enabled: true });
    await waitFor(() => expect(hook.result.current.isError).toBe(true));
    server.use(
      http.get(`${BASE}/me/notifications`, () => {
        requests += 1;
        return listResponse({ items: [], nextBefore: null });
      }),
    );
    await act(async () => {
      await hook.result.current.refetch();
    });
    await waitFor(() => expect(hook.result.current.isError).toBe(false));
    expect(requests).toBe(2);
  });
});
