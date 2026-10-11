import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useBlocker } from "react-router";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ReportUnsaved, type UnsavedReport } from "@/features/settings/unsavedSection";

function within(base: string, pathname: string) {
  return pathname === base || pathname.startsWith(`${base}/`);
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
 */
export function UnsavedChangesGuard({
  base,
  children,
}: Readonly<{ base: string; children: ReactNode }>) {
  const { t } = useTranslation();
  const [unsavedSections, setUnsavedSections] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const report = useCallback<UnsavedReport>(
    (section, unsaved) =>
      setUnsavedSections((sections) => marked(sections, section, unsaved)),
    [],
  );
  const unsaved = unsavedSections.size > 0;

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      unsaved &&
      within(base, currentLocation.pathname) &&
      !within(base, nextLocation.pathname) &&
      nextLocation.pathname !== "/login",
  );

  useEffect(() => {
    if (!unsaved) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [unsaved]);

  return (
    <ReportUnsaved.Provider value={report}>
      {children}
      <ConfirmDialog
        open={blocker.state === "blocked"}
        onOpenChange={(open) => {
          if (!open && blocker.state === "blocked") blocker.reset();
        }}
        title={t("settings.leave.title")}
        description={t("controls.unsaved")}
        confirmLabel={t("settings.leave.confirm")}
        cancelLabel={t("settings.leave.stay")}
        destructive
        onConfirm={() => {
          if (blocker.state === "blocked") blocker.proceed();
        }}
      />
    </ReportUnsaved.Provider>
  );
}
