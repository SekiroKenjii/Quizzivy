import { useEffect, useRef } from "react";

/**
 * useReportUnkeyed tells the page whether the open editor's draft is unkeyed
 * each time that changes, and that it is not once the editor unmounts, so a
 * warning never outlives its pane. The page passes a fresh callback each
 * render, so the latest one is read through a ref.
 */
export function useReportUnkeyed(
  unkeyed: boolean,
  onUnkeyed: (unkeyed: boolean) => void,
): void {
  const report = useRef(onUnkeyed);
  useEffect(() => {
    report.current = onUnkeyed;
  });
  useEffect(() => {
    report.current(unkeyed);
  }, [unkeyed]);
  useEffect(() => () => report.current(false), []);
}
