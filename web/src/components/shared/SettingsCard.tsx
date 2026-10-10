import { useId, type ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * SettingsCard is the Teacher deck's settings card: a section named by its
 * title, with an optional sentence under it and an optional `action` at the
 * end of the header, which a border separates from what the card holds.
 * The card adds no padding around `children`, so a list's rows can run to
 * its edges.
 */
export function SettingsCard({
  title,
  description,
  action,
  className,
  children,
}: Readonly<{
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string | undefined;
  children: ReactNode;
}>) {
  const id = useId();
  return (
    <section
      aria-labelledby={id}
      className={cn("bg-card shadow-card rounded-xl border", className)}
    >
      <div className="flex flex-wrap items-center justify-between gap-2.5 border-b px-4.5 py-4">
        <div className="min-w-0">
          <h2 id={id} className="text-md leading-normal font-semibold">
            {title}
          </h2>
          {description === undefined ? null : (
            <p className="text-muted-fg text-sm leading-normal">{description}</p>
          )}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}
