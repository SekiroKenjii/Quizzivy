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
  it("reads list cursors and limits with the audience, inventing no other parameter", async () => {
    const urls: URL[] = [];
    server.use(
      http.get(`${BASE}/me/notifications`, ({ request }) => {
        urls.push(new URL(request.url));
        expect(request.method).toBe("GET");
        return listResponse();
      }),
    );
    expect(await listNotifications({ audience: "teacher" })).toEqual(page);
    await listNotifications({ audience: "student", before: id(20), limit: 15 });
    expect([...urls[0]!.searchParams]).toEqual([["audience", "teacher"]]);
    expect(Object.fromEntries(urls[1]!.searchParams)).toEqual({
      audience: "student",
      before: id(20),
      limit: "15",
    });
  });
  it("reads summary with the audience and all five preferences and sends all five on PUT", async () => {
    let body: unknown;
    const summaryUrls: URL[] = [];
    server.use(
      http.get(`${BASE}/me/summary`, ({ request }) => {
        summaryUrls.push(new URL(request.url));
        return summaryResponse(9);
      }),
      http.get(`${BASE}/me/notification-preferences`, () => preferenceResponse("get")),
      http.put(`${BASE}/me/notification-preferences`, async ({ request }) => {
        expect(request.method).toBe("PUT");
        body = await request.json();
        return preferenceResponse("put");
      }),
    );
    expect(await getMySummary("student")).toEqual({ unreadNotifications: 9 });
    expect(Object.fromEntries(summaryUrls[0]!.searchParams)).toEqual({
      audience: "student",
    });
    expect(await getNotificationPreferences()).toEqual(preferences);
    expect(await updateNotificationPreferences(preferences)).toEqual(preferences);
    expect(body).toEqual(preferences);
  });
  it("marks all only for absent IDs and preserves explicit IDs, always with the audience", async () => {
    const bodies: unknown[] = [];
    server.use(
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        expect(request.method).toBe("POST");
        bodies.push(await request.json());
        return new HttpResponse(null, { status: 204 });
      }),
    );
    expect(await markNotificationsRead("teacher")).toBeUndefined();
    await markNotificationsRead("student", [id(1), id(2)]);
    expect(bodies).toEqual([
      { audience: "teacher" },
      { audience: "student", ids: [id(1), id(2)] },
    ]);
  });
  it("sends no request for an explicit empty list", async () => {
    const received = vi.fn(() => new HttpResponse(null, { status: 204 }));
    server.use(http.post(`${BASE}/me/notifications/read`, received));
    await markNotificationsRead("teacher", []);
    expect(received).not.toHaveBeenCalled();
  });
  it("sends 250 IDs as ordered batches of100,100,50 with no overlapping requests, each with the audience", async () => {
    const ids = Array.from({ length: 250 }, (_, index) => id(index + 1));
    const batches: string[][] = [];
    const audiences: string[] = [];
    const first = deferred<void>();
    server.use(
      http.post(`${BASE}/me/notifications/read`, async ({ request }) => {
        const body = (await request.json()) as { audience: string; ids: string[] };
        batches.push(body.ids);
        audiences.push(body.audience);
        if (batches.length === 1) await first.promise;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const saving = markNotificationsRead("teacher", ids);
    await waitFor(() => expect(batches).toHaveLength(1));
    expect(batches[0]).toEqual(ids.slice(0, 100));
    first.resolve();
    await saving;
    expect(batches).toEqual([ids.slice(0, 100), ids.slice(100, 200), ids.slice(200)]);
    expect(audiences).toEqual(["teacher", "teacher", "teacher"]);
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
      markNotificationsRead(
        "teacher",
        Array.from({ length: 250 }, (_, i) => id(i + 1)),
      ),
    ).rejects.toMatchObject({ status: 400 });
    expect(batches).toHaveLength(2);
    server.use(http.post(`${BASE}/me/notifications/read`, () => errorResponse(400)));
    await expect(markNotificationsRead("teacher", [id(1)])).rejects.toBeInstanceOf(
      ApiError,
    );
  });
  it("exports the cache keys under the same invalidation prefix, the list and summary per audience", () => {
    expect(notificationKeys.list("teacher")).toEqual([
      "notifications",
      "list",
      "teacher",
    ]);
    expect(notificationKeys.summary("student")).toEqual([
      "notifications",
      "summary",
      "student",
    ]);
    expect({
      all: notificationKeys.all,
      preferences: notificationKeys.preferences,
    }).toEqual({
      all: ["notifications"],
      preferences: ["notifications", "preferences"],
    });
  });
});
