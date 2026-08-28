import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router";
import { useEffect } from "react";
import { queryClient } from "./queryClient";
import { router } from "./router";
import { setSessionLostHandler } from "@/lib/api/client";
import { useBootstrapSession } from "@/features/auth/useSession";
import { useAuthStore } from "@/stores/auth";

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
  // Restores the session before the guards decide anything (§5.4). Runs once,
  // above the router, so a deep link survives the round trip.
  useBootstrapSession();

  useEffect(() => {
    setSessionLostHandler(() => {
      // Only clear the cache if there was a session to clear.
      //
      // The cache clear exists so one user's data cannot be rendered to the
      // next (§5.4). With no session there is no such data -- and clearing
      // anyway wipes queries that are IN FLIGHT, which leaves their components
      // pending forever. That is not hypothetical: an anonymous visitor on
      // /join/:code/confirm bootstraps, gets the 401 a visitor is supposed to
      // get, and the join preview racing alongside it was discarded mid-request.
      // The student saw "Đang tải…" and nothing else, permanently.
      if (useAuthStore.getState().user !== null) {
        queryClient.clear();
      }
      useAuthStore.getState().clearSession();
      // Deliberately NO navigation. Clearing the session is enough:
      // RequireSession redirects anyone standing on a protected route, and it
      // attaches `?next=` while doing it.
      //
      // Navigating from here instead duplicated that decision and got it wrong
      // on the public screens. An anonymous student following a join link
      // bootstraps with `GET /auth/me`, gets the 401 that a visitor with no
      // account is supposed to get, and was thrown off /join onto /login --
      // before ever seeing which class they were invited to. That is the one
      // flow §6.2 exists for.
    });
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
