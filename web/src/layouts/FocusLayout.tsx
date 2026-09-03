import { Outlet } from "react-router";

/**
 * The test-taking shell (§9). Everything that could pull attention away is
 * absent: no nav, no links out.
 *
 * §12: "Student test view spacious, one question centered at max-width ~720px",
 * with `leading-relaxed`. The chrome that belongs here — timer, progress,
 * remaining strikes — is owned by the take-test feature in Phase 3, not by the
 * layout, because it depends on attempt state.
 */
export default function FocusLayout() {
  // The column and nothing else. S-05's header and footer run edge to edge
  // while only the question body is centred at 720px, so the centring belongs
  // to the screen rather than to the shell -- a `main` here wrapped the header
  // and footer too, and left the footer stranded mid-page instead of at the
  // bottom of the viewport.
  //
  // Exactly the viewport: the engine scrolls its paper and pins the rest (S-08).
  return (
    <div className="bg-background flex h-svh flex-col leading-relaxed">
      <Outlet />
    </div>
  );
}
