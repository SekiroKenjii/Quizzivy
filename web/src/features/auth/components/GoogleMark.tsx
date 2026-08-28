/**
 * Google's four-colour "G".
 *
 * A deliberate exception to §12's palette rules, and worth stating why rather
 * than leaving it to look like drift. §12 forbids decorative colour and emoji
 * in UI chrome; this is neither. It is an identifying MARK on a button that
 * hands the user to a third party, and a student scanning a sign-in screen on a
 * phone recognises it faster than they read any label. Google's own branding
 * guidance for sign-in buttons expects the full-colour mark at this size.
 *
 * It is inline rather than an asset: four paths beat a network request that can
 * fail on the one screen a new student sees first, and it inherits nothing from
 * the theme, so dark mode leaves it alone.
 *
 * `aria-hidden` because every caller puts a text label beside it -- announcing
 * "Google" twice is worse than not announcing it once.
 *
 * SIZING IS THE BUTTON'S JOB, and this carries no size class of its own. That
 * is not an omission. `button.tsx` sizes icons with
 * `[&_svg:not([class*='size-'])]:size-4`, dropping to `size-3` on the `xs`
 * variant -- a rule that matches only while the svg has no `size-` class. An
 * earlier version defaulted to `size-4`, which always matched the `:not()` and
 * so opted permanently out of the variant sizing it was trying to agree with:
 * on an `xs` button every other icon shrank and this one did not.
 *
 * The consequence is that a caller OUTSIDE a Button must pass a size, or the
 * browser renders a `viewBox`-only svg at its 300x150 default. All three
 * callers today are buttons.
 */
export function GoogleMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 48 48" className={className} aria-hidden="true" focusable="false">
      <path
        fill="#EA4335"
        d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"
      />
      <path
        fill="#4285F4"
        d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"
      />
      <path
        fill="#FBBC05"
        d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"
      />
      <path
        fill="#34A853"
        d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"
      />
    </svg>
  );
}
