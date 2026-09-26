import { Outlet } from "react-router";
import { useForcedLightTheme } from "@/lib/theme";

/**
 * The test-taking shell (§9). Everything that could pull attention away is
 * absent: no nav, no links out.
 */
export default function FocusLayout() {
  useForcedLightTheme();
  // The column and nothing else.
  return (
    <div className="student-surface bg-background flex h-svh flex-col leading-relaxed">
      <Outlet />
    </div>
  );
}
