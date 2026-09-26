import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import { useEffect } from "react";
import { queryClient } from "./queryClient";
import { router } from "./router";
import { setMaintenanceHandler, setSessionLostHandler } from "@/lib/api/client";
import { useBootstrapSession } from "@/features/auth/useSession";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { useResolvedTheme } from "@/lib/theme";

/**
 * Wires the API client's "the session is gone" and "down for maintenance"
 * signals into the session, the query cache and the app state.
 */
export function AppProviders() {
  useBootstrapSession();
  useResolvedTheme();

  useEffect(() => {
    setSessionLostHandler(() => {
      if (useAuthStore.getState().user !== null) {
        queryClient.clear();
      }
      useAuthStore.getState().clearSession();
    });
    setMaintenanceHandler((window) => {
      useAppState.getState().showOverlay({ kind: "maintenance", window });
    });
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>
  );
}
