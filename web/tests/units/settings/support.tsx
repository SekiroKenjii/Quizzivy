import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import TeacherSettingsPage from "@/features/settings/pages/teacher/TeacherSettingsPage";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";

/** BASE is the API origin the tests' handlers answer on. */
export const BASE = "http://localhost:8080";

/** REQUEST_ID is a request id the error envelope accepts. */
export const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

type Session = components["schemas"]["Session"];

/** THIS_DEVICE is the calling session, as the server lists it first. */
export const THIS_DEVICE: Session = {
  familyId: "019535d9-3df7-79fb-b466-fa907fa18001",
  device: "Mac · Chrome",
  deviceKind: "computer",
  location: "Ho Chi Minh City, VN",
  lastUsedAt: "2026-10-10T09:59:00Z",
  current: true,
};

/** PHONE is another session with a location. */
export const PHONE: Session = {
  familyId: "019535d9-3df7-79fb-b466-fa907fa18002",
  device: "iPhone · Safari",
  deviceKind: "phone",
  location: "Da Nang, VN",
  lastUsedAt: "2026-10-10T08:00:00Z",
  current: false,
};

/** UNKNOWN is another session the server could not name or place. */
export const UNKNOWN: Session = {
  familyId: "019535d9-3df7-79fb-b466-fa907fa18003",
  device: null,
  deviceKind: "unknown",
  location: null,
  lastUsedAt: "2026-10-05T10:00:00Z",
  current: false,
};

/** listsSessions answers `GET /auth/sessions` with `items` and counts the reads. */
export function listsSessions(items: Session[]) {
  const reads = { count: 0 };
  server.use(
    http.get(`${BASE}/auth/sessions`, () => {
      reads.count += 1;
      return contractJson("/auth/sessions", "get", 200, { items });
    }),
  );
  return reads;
}

/** signIn puts `user` in the store as the signed-in account. */
export function signIn(user: components["schemas"]["CurrentUser"]) {
  useAuthStore.getState().setSession("token-a", user);
}

/** renderSettings renders the Settings page at `path` with a query client and a toaster. */
export function renderSettings(path = "/teacher/settings") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/settings/:section?", element: <TeacherSettingsPage /> }],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { router, client };
}
