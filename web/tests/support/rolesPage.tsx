import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import RolesPage from "@/features/roles/pages/admin/RolesPage";
import { Toaster } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { adminUser } from "./fixtures";

type User = components["schemas"]["CurrentUser"];

/**
 * renderRolesPage signs `user` in and renders the Roles & permissions page at
 * `at` in a memory router with a toaster, returning the router, the query
 * client and a user-event instance.
 */
export function renderRolesPage(user: User = adminUser, at = "/admin/roles") {
  useAuthStore.getState().setSession("token", user);
  const router = createMemoryRouter(
    [{ path: "/admin/roles", element: <RolesPage /> }],
    {
      initialEntries: [at],
    },
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Toaster />
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, client, user: userEvent.setup() };
}
