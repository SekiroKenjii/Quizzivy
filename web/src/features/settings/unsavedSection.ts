import { createContext, useContext, useEffect } from "react";

/** UnsavedReport records whether one section holds unsaved changes. */
export type UnsavedReport = (section: string, unsaved: boolean) => void;

/**
 * ReportUnsaved carries the enclosing UnsavedChangesGuard's report; outside
 * a guard it does nothing.
 */
export const ReportUnsaved = createContext<UnsavedReport>(() => undefined);

/**
 * useUnsavedSection tells the enclosing UnsavedChangesGuard whether
 * `section` holds edits that are not saved yet, and withdraws the claim when
 * the section unmounts. Outside a guard it does nothing.
 */
export function useUnsavedSection(section: string, unsaved: boolean) {
  const report = useContext(ReportUnsaved);
  useEffect(() => {
    report(section, unsaved);
    return () => report(section, false);
  }, [report, section, unsaved]);
}
