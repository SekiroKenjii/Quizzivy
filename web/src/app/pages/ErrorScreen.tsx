import type { ReactNode } from "react";

/**
 * The deck's S-11 shape for a screen that reports a failure: centred, high on
 * the page, an optional muted glyph, then what happened and what to do next.
 *
 * Shared by 403, 404 and the error boundary because the deck draws them as one
 * shape — a student who hits two of them should not have to re-read the layout.
 */
export function ErrorScreen({
  icon,
  title,
  body,
  children,
}: {
  icon?: ReactNode;
  title: string;
  body: string;
  children?: ReactNode;
}) {
  return (
    <main className="mx-auto max-w-sm p-4 pt-16 text-center">
      {icon}
      <h1 className={`text-lg font-semibold tracking-tight ${icon ? "mt-4" : ""}`}>
        {title}
      </h1>
      <p className="text-muted-foreground mt-2 text-sm leading-relaxed">{body}</p>
      {children}
    </main>
  );
}
