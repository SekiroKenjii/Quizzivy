import { Outlet } from "react-router";

/**
 * FocusLayout is the test-taking shell (§9): the column and nothing else, with
 * no nav and no links out. It follows the theme preference, light or dark. Below
 * 1024px `.student-surface` keeps every button in it at least 44px each way;
 * a control drawn to the deck's size opts out, which leaves the floor on the
 * buttons the deck does not draw.
 */
export default function FocusLayout() {
  return (
    <div className="student-surface bg-bg text-fg @container/student flex h-svh flex-col leading-relaxed">
      <Outlet />
    </div>
  );
}
