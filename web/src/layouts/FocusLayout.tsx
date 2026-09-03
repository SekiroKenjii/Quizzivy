import { Outlet } from "react-router";

/**
 * The test-taking shell (§9). Everything that could pull attention away is
 * absent: no nav, no links out.
 */
export default function FocusLayout() {
  // The column and nothing else.
  return (
    <div className="bg-background flex h-svh flex-col leading-relaxed">
      <Outlet />
    </div>
  );
}
