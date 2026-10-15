import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { requestSetPasswordLink } from "@/features/auth/api";
import { failureMessage } from "@/lib/api/errors";
import { authStore } from "@/stores/auth";

/**
 * SetPasswordLinkRequest is where asking for a set-password link stands: not
 * asked, on its way, sent, or refused with the server's own words.
 */
export type SetPasswordLinkRequest =
  | { state: "idle" }
  | { state: "sending" }
  | { state: "sent" }
  | { state: "failed"; message: string };

/**
 * useSetPasswordLinkRequest asks the server to email the signed-in account a
 * link to set a password. `send` does nothing while a request is on its way,
 * and an answer that arrives for another account or after unmounting is
 * dropped.
 */
export function useSetPasswordLinkRequest() {
  const { t } = useTranslation();
  const [request, setRequest] = useState<SetPasswordLinkRequest>({ state: "idle" });
  const sending = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const send = useCallback(async () => {
    if (sending.current) return;
    sending.current = true;
    setRequest({ state: "sending" });
    const lease = authStore.captureActor();
    try {
      await requestSetPasswordLink();
      if (mounted.current && authStore.isCurrent(lease)) setRequest({ state: "sent" });
    } catch (cause) {
      if (mounted.current && authStore.isCurrent(lease))
        setRequest({
          state: "failed",
          message: failureMessage(cause, t("api.failed")),
        });
    } finally {
      sending.current = false;
    }
  }, [t]);

  return { request, send };
}
