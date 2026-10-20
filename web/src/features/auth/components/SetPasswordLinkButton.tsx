import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import type { SetPasswordLinkRequest } from "@/features/auth/setPasswordLink";

/**
 * SetPasswordLinkButton asks for an emailed set-password link. While the
 * request is on its way it stays in the tab order, marked `aria-disabled`;
 * once a link is sent it offers to send another.
 */
export function SetPasswordLinkButton({
  state,
  className,
  onSend,
}: Readonly<{
  state: SetPasswordLinkRequest["state"];
  className?: string;
  onSend: () => void;
}>) {
  const { t } = useTranslation();
  const sending = state === "sending";
  return (
    <Button
      type="button"
      variant="outline"
      className={className}
      aria-disabled={sending || undefined}
      aria-busy={sending || undefined}
      onClick={() => {
        if (!sending) onSend();
      }}
    >
      {t(setPasswordLinkLabel(state))}
    </Button>
  );
}

function setPasswordLinkLabel(state: SetPasswordLinkRequest["state"]) {
  if (state === "sending") return "settings.setPasswordLink.sending";
  return state === "sent"
    ? "settings.setPasswordLink.sendAgain"
    : "settings.setPasswordLink.send";
}
