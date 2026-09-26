import type { ReactNode } from "react";
import { SystemFrame } from "@/app/pages/SystemFrame";

/**
 * The shape the failure screens share: the system frame with the drawing in
 * the panel and the page's title, body, evidence and footnote in the column.
 */
export function ErrorScreen({
  art,
  title,
  body,
  footer,
  children,
}: Readonly<{
  /** The panel drawing from `errorArt`. */
  art: ReactNode;
  title: string;
  body: string;
  /** The line under the card explaining what to do about it. */
  footer?: ReactNode;
  /** Evidence and actions: what this particular failure can offer. */
  children?: ReactNode;
}>) {
  return (
    <SystemFrame art={art}>
      <h1 className="text-h1 text-balance">{title}</h1>
      <p className="text-muted-fg text-body mt-2 text-pretty">{body}</p>
      {children}
      {footer && (
        <p className="text-muted-fg mt-6 text-center text-sm text-pretty">{footer}</p>
      )}
    </SystemFrame>
  );
}

/**
 * The actions block. Ranked, stacked, full width — never side by side, because
 * on a phone two half-width buttons are two small targets and the ranking stops
 * being visible.
 */
export function ErrorActions({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <div className="*:rounded-ctl in-data-[scale=deck]:*:text-md mt-5 flex flex-col gap-2 *:h-11 *:w-full in-data-[scale=deck]:*:px-4.5 lg:*:h-9.5 in-data-[scale=deck]:lg:*:text-base">
      {children}
    </div>
  );
}
