import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, Link, Outlet, RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import SystemSettingsPage from "@/features/organization/pages/admin/SystemSettingsPage";

/** renderSettings renders System settings at `path`, with a link out to check what leaving asks. */
export function renderSettings(path = "/admin/settings") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        element: (
          <>
            <Link to="/admin">Tổng quan</Link>
            <Outlet />
          </>
        ),
        children: [
          { path: "/admin/settings/:section?", element: <SystemSettingsPage /> },
          { path: "/admin", element: <h1>Trang tổng quan</h1> },
        ],
      },
    ],
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
