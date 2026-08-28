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
  return (
    <div className="bg-background flex min-h-svh flex-col leading-relaxed">
      <main
        className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6"
        style={{ paddingBottom: "max(1.5rem, env(safe-area-inset-bottom))" }}
      >
        <Outlet />
      </main>
    </div>
  );
}
