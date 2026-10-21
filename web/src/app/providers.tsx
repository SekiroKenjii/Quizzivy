import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import { useEffect } from "react";
import { queryClient } from "./queryClient";
import { router } from "./router";
import {
  setMaintenanceHandler,
  setPasswordChangeHandler,
  setSessionLostHandler,
} from "@/lib/api/client";
import { useBootstrapSession } from "@/features/auth/useSession";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { useResolvedTheme } from "@/lib/theme";
import { AppFrame } from "./AppFrame";

/**
 * Wires the API client's "the session is gone", "down for maintenance" and
 * "change the temporary password" signals into the app state: a signed-in user
 * whose session is refused gets the "sign in again" overlay, a 503
 * `MAINTENANCE` the maintenance one, and a 403 `PASSWORD_CHANGE_REQUIRED` marks
 * the user so the session guard sends them to Change password.
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
    setPasswordChangeHandler(() => {
      useAuthStore.getState().requirePasswordChange();
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
