import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router";
import { useEffect } from "react";
import { queryClient } from "./queryClient";
import { router } from "./router";
import { setSessionLostHandler } from "@/lib/api/client";

/**
 * Wires the API client's "the session is gone" signal into the router and the
 * query cache.
 *
 * §5.4 spells out what has to happen: clear the store (the client already
 * does), `queryClient.clear()`, then go to /login. The cache clear is not
 * optional — without it, data fetched as one user stays in memory and can be
 * rendered to whoever signs in next.
 *
 * Navigating through the router rather than `window.location` keeps it an SPA
 * transition; the client falls back to a hard redirect if this never runs.
 */
export function AppProviders() {
  useEffect(() => {
    setSessionLostHandler(() => {
      queryClient.clear();
      if (router.state.location.pathname !== "/login") {
        void router.navigate("/login", { replace: true });
      }
    });
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
