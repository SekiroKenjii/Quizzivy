import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "@/components/ui/sonner";

/**
 * joinLinkOf is the one join link format (DG-01): the app's origin, `/join/`
 * and the code as it is grouped, dash kept, so a link and a card read alike.
 * The join page reads it with or without the dash.
 */
export function joinLinkOf(code: string): string {
  return `${window.location.origin}/join/${code}`;
}

/**
 * useCopyJoinLink returns a function that copies a code's join link and
 * says so in a toast naming the link, or that the copy failed. Neither the
 * code nor the link is kept.
 */
export function useCopyJoinLink(): (code: string) => void {
  const { t } = useTranslation();
  return useCallback(
    (code: string) => {
      const link = joinLinkOf(code);
      void Promise.resolve()
        .then(() => navigator.clipboard.writeText(link))
        .then(
          () => toast(t("classes.linkCopied", { url: link })),
          () => toast.error(t("classes.copyFailed")),
        );
    },
    [t],
  );
}
