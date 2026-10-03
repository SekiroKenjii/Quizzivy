import { createElement, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Ban } from "lucide-react";
import { notify } from "@/lib/toast";

const KINDS = ["copy", "cut", "paste"] as const;
const TOAST_ID = "integrity-clipboard";

/**
 * useClipboardNotice shows the deck's danger toast, "Copy and paste are
 * turned off in this test", each time the student copies, cuts or pastes
 * while `blocked`. It only tells the student. The integrity monitor is what
 * records the event and stops it, and it does both whether or not this hook
 * runs. A repeat replaces the toast on screen instead of queueing another.
 */
export function useClipboardNotice(blocked: boolean): void {
  const { t } = useTranslation();
  useEffect(() => {
    if (!blocked) return;
    const tell = () => {
      notify.error(t("integrity.copyPasteOff"), {
        id: TOAST_ID,
        icon: createElement(Ban, {
          "aria-hidden": true,
          className: "text-danger size-4",
        }),
      });
    };
    for (const kind of KINDS) document.addEventListener(kind, tell);
    return () => {
      for (const kind of KINDS) document.removeEventListener(kind, tell);
    };
  }, [blocked, t]);
}
