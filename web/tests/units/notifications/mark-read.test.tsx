import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@tests/support/server";
import { useNotificationList } from "@/features/notifications/useNotificationList";
import { useNotificationSummary } from "@/features/notifications/useNotificationSummary";
import { useNotificationPreferences } from "@/features/notifications/useNotificationPreferences";
import { useMarkNotificationsRead } from "@/features/notifications/useMarkNotificationsRead";
import { notificationKeys } from "@/features/notifications/api";
import {
  BASE,
  harness,
  id,
  listResponse,
  summaryResponse,
  preferenceResponse,
  deferred,
  errorResponse,
} from "./support";

describe("settled notification read mutation", () => {
  it.each([false, true])(
    "invalidates list,summary,preferences after failure=%s without an optimistic read",
    async (fail) => {
      let lists = 0;
      let summaries = 0;
      let preferences = 0;
      let body: unknown;
      const gate = deferred<void>();
      server.use(
        http.get(`${BASE}/me/notifications`, () => {
          lists += 1;
          return listResponse();
        }),
        http.get(`${BASE}/me/summary`, () => {
          summaries += 1;
          return summaryResponse(3);
        }),
        http.get(`${BASE}/me/notification-preferences`, () => {
          preferences += 1;
          return preferenceResponse("get");
        }),
        http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
          body = await request.json();
          await gate.promise;
          return fail ? errorResponse(400) : new HttpResponse(null, { status: 204 });
        }),
      );
      const setup = harness();
      const hook = renderHook(
        () => ({
          list: useNotificationList(),
          summary: useNotificationSummary(),
          preferences: useNotificationPreferences(),
          mark: useMarkNotificationsRead(),
        }),
        setup,
      );
      await waitFor(() => expect(hook.result.current.summary.unread).toBe(3));
      await waitFor(() => expect(hook.result.current.list.items).toHaveLength(20));
      await waitFor(() =>
        expect(hook.result.current.preferences.query.isSuccess).toBe(true),
      );
      const originalCache = setup.client.getQueryData(notificationKeys.list);
      act(() => hook.result.current.mark.mutate([id(1)]));
      await waitFor(() => expect(body).toEqual({ ids: [id(1)] }));
      expect(setup.client.getQueryData(notificationKeys.list)).toBe(originalCache);
      expect(hook.result.current.list.items[0]!.readAt).toBeNull();
      expect(hook.result.current.summary.unread).toBe(3);
      expect([lists, summaries, preferences]).toEqual([1, 1, 1]);
      gate.resolve();
      await waitFor(() => expect([lists, summaries, preferences]).toEqual([2, 2, 2]));
      await waitFor(() => expect(hook.result.current.mark.isPending).toBe(false));
      expect(hook.result.current.mark.isError).toBe(fail);
    },
  );
  it("marks all through the public mutation with an absent ID argument", async () => {
    let body: unknown;
    server.use(
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        body = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const hook = renderHook(() => useMarkNotificationsRead(), harness());
    await act(async () => {
      await hook.result.current.mutateAsync(undefined);
    });
    expect(body).toEqual({});
  });
});
