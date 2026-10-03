import { Outlet } from "react-router";
import { DeckScale } from "@/components/ui/deck-scale";

/**
 * FocusLayout is the test-taking shell (§9): the column and nothing else, with
 * no nav and no links out. It is a deck surface, so the engine reads the
 * deck's geometry and follows the theme preference, light or dark. The 44px
 * floor of `.student-surface` stays for the parts of the engine not yet
 * rebuilt to the deck; a control drawn to the deck's size opts out of it.
 */
export default function FocusLayout() {
  return (
    <DeckScale className="student-surface bg-bg text-fg @container/student flex h-svh flex-col leading-relaxed">
      <Outlet />
    </DeckScale>
  );
}
