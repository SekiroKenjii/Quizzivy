import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useBlocker } from "react-router";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import {
  ConfirmMove,
  ReportUnsaved,
  type ConfirmMoveFn,
  type UnsavedReport,
} from "@/features/settings/unsavedSection";

function within(base: string, pathname: string) {
  return pathname === base || pathname.startsWith(`${base}/`);
}

type HeldMove = { run: () => void; spare: (section: string) => boolean };

/** holds says some section that `spare` does not accept is unsaved. */
function holds(sections: ReadonlySet<string>, spare: (section: string) => boolean) {
  return [...sections].some((section) => !spare(section));
}

function marked(
  sections: ReadonlySet<string>,
  section: string,
  unsaved: boolean,
): ReadonlySet<string> {
  if (sections.has(section) === unsaved) return sections;
  const next = new Set(sections);
  if (unsaved) next.add(section);
  else next.delete(section);
  return next;
}

/**
 * UnsavedChangesGuard asks before a navigation leaves `base` while any
 * section under it reports unsaved changes: Stay keeps the page and returns
 * focus to what started the navigation, Leave goes on and drops the edits.
 * Moving between addresses under `base` is never blocked, since the sections
 * stay mounted, and neither is a move to /login, which signing out makes. A
 * reload or a closed tab gets the browser's own prompt meanwhile. A
 * navigation judged after the URL has already left `base` is let through, so
 * a Back pressed while the departing page is still mounted is not reverted.
 * An in-page move made through ConfirmMove asks the same question. A
 * question asked while a save was in flight goes away once the save lands:
 * the move or navigation goes on, since nothing is left to lose.
 * `copy` replaces the Settings wording for a guard elsewhere.
 */
export function UnsavedChangesGuard({
  base,
  copy,
  children,
}: Readonly<{
  base: string;
  copy?: { title: string; description: string; confirm: string; stay: string };
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  const [unsavedSections, setUnsavedSections] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const current = useRef<ReadonlySet<string>>(unsavedSections);
  const [pending, setPending] = useState<HeldMove | null>(null);
  const held = useRef<HeldMove | null>(null);
  const hold = useCallback((next: HeldMove | null) => {
    held.current = next;
    setPending(next);
  }, []);
  const report = useCallback<UnsavedReport>(
    (section, unsaved) => {
      current.current = marked(current.current, section, unsaved);
      setUnsavedSections(current.current);
      if (unsaved) return;
      const waiting = held.current;
      if (waiting !== null && !holds(current.current, waiting.spare)) {
        hold(null);
        waiting.run();
      }
    },
    [hold],
  );
  const unsaved = unsavedSections.size > 0;
  const move = pending?.run ?? null;
  const confirmMove = useCallback<ConfirmMoveFn>(
    (next, spare = () => false) => {
      if (holds(current.current, spare)) hold({ run: next, spare });
      else next();
    },
    [hold],
  );

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      unsaved &&
      within(base, currentLocation.pathname) &&
      !within(base, nextLocation.pathname) &&
      nextLocation.pathname !== "/login",
  );
  useEffect(() => {
    if (blocker.state === "blocked" && !unsaved) blocker.proceed();
  }, [blocker, unsaved]);

  useEffect(() => {
    if (!unsaved) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [unsaved]);

  return (
    <ReportUnsaved.Provider value={report}>
      <ConfirmMove.Provider value={confirmMove}>{children}</ConfirmMove.Provider>
      <ConfirmDialog
        open={blocker.state === "blocked" || move !== null}
        onOpenChange={(open) => {
          if (open) return;
          hold(null);
          if (blocker.state === "blocked") blocker.reset();
        }}
        title={copy?.title ?? t("settings.leave.title")}
        description={copy?.description ?? t("controls.unsaved")}
        confirmLabel={copy?.confirm ?? t("settings.leave.confirm")}
        cancelLabel={copy?.stay ?? t("settings.leave.stay")}
        destructive
        onConfirm={() => {
          if (blocker.state === "blocked") blocker.proceed();
          move?.();
          hold(null);
        }}
      />
    </ReportUnsaved.Provider>
  );
}
