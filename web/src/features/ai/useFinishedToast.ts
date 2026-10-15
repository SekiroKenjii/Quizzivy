import { useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { leaveToast, notify } from "@/lib/toast";

const FINISHED_TOAST_MS = 8000;

/**
 * useFinishedToast returns `announce`, which a panel calls when a run
 * settles, answered or failed. While the panel's drawer is showing it says
 * nothing. Otherwise it raises a toast with the panel's `title` and an Open
 * action that calls `onReopen`, or, while another drawer is showing (a modal
 * keeps a toast from being clicked), waits until none is. The toast goes when
 * the drawer opens and when the panel is unmounted.
 */
export function useFinishedToast({
  open,
  aside,
  title,
  onReopen,
}: Readonly<{
  open: boolean;
  aside: boolean;
  title: string;
  onReopen: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const showing = useRef(open);
  const crowded = useRef(aside);
  const raised = useRef<string | number | null>(null);
  const owed = useRef<"done" | "failed" | null>(null);

  const raise = useCallback(
    (outcome: "done" | "failed") => {
      const options = {
        description: t(`ai.finished.${outcome}`),
        duration: FINISHED_TOAST_MS,
        ...(onReopen !== undefined && {
          action: {
            label: t("ai.finished.open"),
            onClick: () => {
              leaveToast();
              onReopen();
            },
          },
        }),
      };
      raised.current =
        outcome === "failed"
          ? notify.warning(title, options)
          : notify.success(title, options);
    },
    [onReopen, t, title],
  );

  useEffect(() => {
    showing.current = open;
    crowded.current = aside;
    if (open) {
      owed.current = null;
      if (raised.current !== null) notify.dismiss(raised.current);
      raised.current = null;
    } else if (!aside && owed.current !== null) {
      raise(owed.current);
      owed.current = null;
    }
  }, [open, aside, raise]);

  useEffect(
    () => () => {
      if (raised.current !== null) notify.dismiss(raised.current);
    },
    [],
  );

  return useCallback(
    (outcome: "done" | "failed") => {
      if (showing.current) return;
      if (crowded.current) owed.current = outcome;
      else raise(outcome);
    },
    [raise],
  );
}
