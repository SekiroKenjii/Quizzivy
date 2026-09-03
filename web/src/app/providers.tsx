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
 */
export function AppProviders() {
  useBootstrapSession();

  useEffect(() => {
    setSessionLostHandler(() => {
      if (useAuthStore.getState().user !== null) {
        queryClient.clear();
      }
      useAuthStore.getState().clearSession();
    });
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
