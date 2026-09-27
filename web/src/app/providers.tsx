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
import { AppFrame } from "./AppFrame";

/**
 * Wires the API client's "the session is gone" and "down for maintenance"
 * signals into the app state: a signed-in user whose session is refused gets
 * the "sign in again" overlay, and a 503 `MAINTENANCE` the maintenance one.
 * The app frame renders beside the router.
 */
export function AppProviders() {
  useBootstrapSession();
  useResolvedTheme();

  useEffect(() => {
    setSessionLostHandler(() => {
      if (useAuthStore.getState().expired) {
        useAppState.getState().showOverlay({ kind: "expired" });
      }
    });
    setMaintenanceHandler((window) => {
      useAppState.getState().showOverlay({ kind: "maintenance", window });
    });
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <AppFrame router={router} />
      <Toaster />
    </QueryClientProvider>
  );
}
