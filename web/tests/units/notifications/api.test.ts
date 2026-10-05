import { describe, expect, it, vi } from "vitest";
import { waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { server } from "@tests/support/server";
import { ApiError } from "@/lib/api/errors";
import {
  getMySummary,
  getNotificationPreferences,
  listNotifications,
  markNotificationsRead,
  notificationKeys,
  updateNotificationPreferences,
} from "@/features/notifications/api";
import {
  BASE,
  id,
  page,
  preferences,
  listResponse,
  summaryResponse,
  preferenceResponse,
  errorResponse,
  deferred,
} from "./support";

describe("notification wire API", () => {
  it("reads list cursors and limits without inventing absent query parameters", async () => {
    const urls: URL[] = [];
    server.use(
      http.get(`${BASE}/me/notifications`, ({ request }) => {
        urls.push(new URL(request.url));
        expect(request.method).toBe("GET");
        return listResponse();
      }),
    );
    expect(await listNotifications({})).toEqual(page);
    await listNotifications({ before: id(20), limit: 15 });
    expect([...urls[0]!.searchParams]).toEqual([]);
    expect(Object.fromEntries(urls[1]!.searchParams)).toEqual({
      before: id(20),
      limit: "15",
    });
  });
  it("reads summary and all five preferences and sends all five on PUT", async () => {
    let body: unknown;
    server.use(
      http.get(`${BASE}/me/summary`, () => summaryResponse(9)),
      http.get(`${BASE}/me/notification-preferences`, () => preferenceResponse("get")),
      http.put(`${BASE}/me/notification-preferences`, async ({ request }) => {
        expect(request.method).toBe("PUT");
        body = await request.json();
        return preferenceResponse("put");
      }),
    );
    expect(await getMySummary()).toEqual({ unreadNotifications: 9 });
    expect(await getNotificationPreferences()).toEqual(preferences);
    expect(await updateNotificationPreferences(preferences)).toEqual(preferences);
    expect(body).toEqual(preferences);
  });
  it("marks all only for absent IDs and preserves explicit IDs", async () => {
    const bodies: unknown[] = [];
    server.use(
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        expect(request.method).toBe("POST");
        bodies.push(await request.json());
        return new HttpResponse(null, { status: 204 });
      }),
    );
    expect(await markNotificationsRead()).toBeUndefined();
    await markNotificationsRead([id(1), id(2)]);
    expect(bodies).toEqual([{}, { ids: [id(1), id(2)] }]);
  });
  it("sends no request for an explicit empty list", async () => {
    const received = vi.fn(() => new HttpResponse(null, { status: 204 }));
    server.use(http.post(`${BASE}/me/notifications/read`, received));
    await markNotificationsRead([]);
    expect(received).not.toHaveBeenCalled();
  });
  it("sends 250 IDs as ordered batches of100,100,50 with no overlapping requests", async () => {
    const ids = Array.from({ length: 250 }, (_, index) => id(index + 1));
    const batches: string[][] = [];
    const first = deferred<void>();
    server.use(
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        const body = (await request.json()) as { ids: string[] };
        batches.push(body.ids);
        if (batches.length === 1) await first.promise;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const saving = markNotificationsRead(ids);
    await waitFor(() => expect(batches).toHaveLength(1));
    expect(batches[0]).toEqual(ids.slice(0, 100));
    first.resolve();
    await saving;
    expect(batches).toEqual([ids.slice(0, 100), ids.slice(100, 200), ids.slice(200)]);
  });
  it("rejects a400 as ApiError and stops before later batches after failure", async () => {
    const batches: unknown[] = [];
    server.use(
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        batches.push(await request.json());
        return batches.length === 1
          ? new HttpResponse(null, { status: 204 })
          : errorResponse(400);
      }),
    );
    await expect(
      markNotificationsRead(Array.from({ length: 250 }, (_, i) => id(i + 1))),
    ).rejects.toMatchObject({ status: 400 });
    expect(batches).toHaveLength(2);
    server.use(http.post(`${BASE}/me/notifications/read`, () => errorResponse(400)));
    await expect(markNotificationsRead([id(1)])).rejects.toBeInstanceOf(ApiError);
  });
  it("exports the four cache keys under the same invalidation prefix", () => {
    expect(notificationKeys).toEqual({
      all: ["notifications"],
      list: ["notifications", "list"],
      summary: ["notifications", "summary"],
      preferences: ["notifications", "preferences"],
    });
  });
});
