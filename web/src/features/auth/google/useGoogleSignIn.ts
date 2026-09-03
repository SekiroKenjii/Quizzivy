import { useState } from "react";
import { useTranslation } from "react-i18next";
import { buildAuthorizationRequest, rememberPending } from "./pkce";

/** Public config (§5.3). Absent in a deployment without Google sign-in. */
const CLIENT_ID = import.meta.env["VITE_GOOGLE_CLIENT_ID"] as string | undefined;

export function googleSignInAvailable(): boolean {
  return typeof CLIENT_ID === "string" && CLIENT_ID.length > 0;
}

/** Starts the §5.3 flow by navigating the whole tab to Google. */
export function useGoogleSignIn() {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function start(
    options: {
      mode?: "signin" | "link";
      next?: string | undefined;
      joinCode?: string | undefined;
    } = {},
  ) {
    if (!CLIENT_ID) {
      setError(t("login.googleUnavailable"));
      return;
    }
    setError(null);
    setPending(true);
    try {
      const request = await buildAuthorizationRequest({
        clientId: CLIENT_ID,
        mode: options.mode ?? "signin",
        next: options.next,
        joinCode: options.joinCode,
      });
      rememberPending(request.pending);
      window.location.assign(request.url);
    } catch {
      setError(t("login.googleUnavailable"));
      setPending(false);
    }
  }

  return { start, error, pending };
}
