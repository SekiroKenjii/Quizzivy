import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { http } from "msw";
import { server } from "@tests/support/server";
import { useNotificationPreferences } from "@/features/notifications/useNotificationPreferences";
import { withPreference } from "@/features/notifications/preferences";
import { notificationKeys } from "@/features/notifications/api";
import {
  BASE,
  harness,
  preferences,
  preferenceResponse,
  deferred,
  errorResponse,
} from "./support";

describe("notification preferences hook", () => {
  it("reads five, preserves unseen student rows on teacher edits, and caches the actual PUT answer", async () => {
    const edited = withPreference(preferences, "attempt.flagged", { inApp: false });
    const answer = withPreference(edited, "result.ready", { email: true });
    let body: unknown;
    const gate = deferred<void>();
    server.use(
      http.get(`${BASE}/me/notification-preferences`, () => preferenceResponse("get")),
      http.put(`${BASE}/me/notification-preferences`, async ({ request }) => {
        body = await request.json();
        await gate.promise;
        return preferenceResponse("put", answer);
      }),
    );
    const setup = harness();
    const hook = renderHook(() => useNotificationPreferences(), setup);
    await waitFor(() => expect(hook.result.current.query.data).toEqual(preferences));
    act(() => hook.result.current.save.mutate(edited));
    await waitFor(() => expect(body).toEqual(edited));
    expect(setup.client.getQueryData(notificationKeys.preferences)).toEqual(
      preferences,
    );
    expect((body as typeof preferences).slice(3)).toEqual(preferences.slice(3));
    gate.resolve();
    await waitFor(() => expect(hook.result.current.query.data).toEqual(answer));
    expect(setup.client.getQueryData(notificationKeys.preferences)).toEqual(answer);
  });
  it("leaves the last read rows intact on failed save", async () => {
    server.use(
      http.get(`${BASE}/me/notification-preferences`, () => preferenceResponse("get")),
      http.put(`${BASE}/me/notification-preferences`, () => errorResponse(400)),
    );
    const setup = harness();
    const hook = renderHook(() => useNotificationPreferences(), setup);
    await waitFor(() => expect(hook.result.current.query.isSuccess).toBe(true));
    act(() =>
      hook.result.current.save.mutate(
        withPreference(preferences, "result.ready", { inApp: false }),
      ),
    );
    await waitFor(() => expect(hook.result.current.save.isError).toBe(true));
    expect(setup.client.getQueryData(notificationKeys.preferences)).toEqual(
      preferences,
    );
  });
});
