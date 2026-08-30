import type { ReactNode } from "react";
import { AuthLayout } from "@/features/auth/AuthLayout";

/**
 * The shape all three failure screens share (E-01..E-03): the same two-panel
 * layout login uses, with the drawing in the panel and the answer in the card.
 *
 * One shape for three screens is the whole argument. A student who has read the
 * 404 can read the 403 without re-learning where anything is, and below `lg`
 * all three collapse the same way because `AuthLayout` collapses.
 *
 * The status number never appears. It means nothing to a fifteen-year-old and
 * everything to us, so it stays in the console and out of the copy.
 */
export function ErrorScreen({
  art,
  title,
  body,
  footer,
  children,
}: {
  /** The panel drawing from `errorArt`. */
  art: ReactNode;
  title: string;
  body: string;
  /** The line under the card explaining what to do about it. */
  footer?: ReactNode;
  /** Evidence and actions: what this particular failure can offer. */
  children?: ReactNode;
}) {
  return (
    <AuthLayout art={art} footer={footer}>
      <h1 className="text-lg font-semibold tracking-tight lg:text-xl">{title}</h1>
      <p className="text-muted-foreground mt-2 text-sm leading-relaxed">{body}</p>
      {children}
    </AuthLayout>
  );
}

/**
 * The actions block. Ranked, stacked, full width — never side by side, because
 * on a phone two half-width buttons are two small targets and the ranking stops
 * being visible.
 *
 * Taller on a phone and default height from `lg` up, which is what E-01 and
 * E-04 draw: 44px clears the touch target on the screen where these are pressed
 * with a thumb.
 */
export function ErrorActions({ children }: { children: ReactNode }) {
  return <div className="mt-5 space-y-2 *:h-11 *:w-full lg:*:h-9">{children}</div>;
}
