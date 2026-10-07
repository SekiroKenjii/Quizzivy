import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import AdminSettingsPage from "@/features/auth/pages/AdminSettingsPage";
import { useAuthStore } from "@/stores/auth";
import type { components } from "@/lib/api/schema";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

afterEach(() => {
  cleanup();
  useAuthStore.getState().clearSession();
});

function openPreferencesAs(signedIn: components["schemas"]["CurrentUser"]) {
  useAuthStore.getState().setUser(signedIn);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/settings/:section?", element: <AdminSettingsPage /> }],
    { initialEntries: ["/teacher/settings/preferences"] },
  );
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe("the teacher Settings page", () => {
  it("the API reference shows only to who may open it", () => {
    const teacher = openPreferencesAs(teacherUser);
    expect(screen.getByRole("heading", { name: "Ngôn ngữ" })).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Tài liệu API", hidden: true }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Mở tài liệu API", hidden: true }),
    ).toBeNull();
    teacher.unmount();
    useAuthStore.getState().clearSession();

    openPreferencesAs(adminUser);
    expect(screen.getByRole("heading", { name: "Ngôn ngữ" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Tài liệu API" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mở tài liệu API" })).toBeInTheDocument();
  });
});
