import { useEffect, useRef, type ReactNode } from "react";
import { Link } from "react-router";
import type { LucideIcon } from "lucide-react";

import { revealWithin } from "@/lib/revealWithin";
import { cn } from "@/lib/utils";

/**
 * SettingsSection is one section of a SettingsLayout: its id, the label and
 * icon of its link, the address the link goes to, and what the section shows.
 */
export type SettingsSection = {
  id: string;
  label: string;
  icon: LucideIcon;
  to: string;
  content: ReactNode;
};

/**
 * SettingsLayout is the Teacher deck's settings frame: a navigation of one
 * link per section beside the current section's content. From 768px the
 * links are a column of 200px, or as wide as the longest label; below it
 * they are one row that scrolls sideways and brings the current link into
 * view. Every section's content is rendered and stays mounted, hidden unless
 * it is the current one, so a form keeps what was typed while another
 * section is shown. An `active` that names no section shows the first.
 */
export function SettingsLayout({
  label,
  active,
  sections,
}: Readonly<{
  label: string;
  active: string;
  sections: readonly SettingsSection[];
}>) {
  const nav = useRef<HTMLElement>(null);
  const current = sections.some((section) => section.id === active)
    ? active
    : sections[0]?.id;

  useEffect(() => {
    const frame = nav.current;
    const link = frame?.querySelector('[aria-current="page"]');
    if (frame && link) revealWithin(frame, link);
  }, [current]);

  return (
    <div className="flex flex-wrap items-start gap-6">
      <nav
        ref={nav}
        aria-label={label}
        onFocus={(event) => revealWithin(event.currentTarget, event.target)}
        className="flex max-w-full min-w-0 flex-[0_1_100%] [scrollbar-width:none] gap-0.5 overflow-x-auto min-[768px]:min-w-50 min-[768px]:flex-none min-[768px]:flex-col [&::-webkit-scrollbar]:hidden"
      >
        {sections.map(({ id, label: name, icon: Icon, to }) => (
          <Link
            key={id}
            to={to}
            aria-current={id === current ? "page" : undefined}
            className={cn(
              "text-ui hover:bg-hover hover:text-fg flex h-9 flex-none items-center gap-2.5 rounded-md px-3 leading-4.5 whitespace-nowrap -outline-offset-2!",
              id === current ? "bg-hover text-fg font-medium" : "text-muted-fg",
            )}
          >
            <Icon aria-hidden="true" className="size-4 flex-none" />
            {name}
          </Link>
        ))}
      </nav>
      <div className="min-w-0 flex-[1_1_480px]">
        {sections.map(({ id, content }) => (
          <div
            key={id}
            data-section={id}
            hidden={id !== current}
            className="flex flex-col gap-3.5"
          >
            {content}
          </div>
        ))}
      </div>
    </div>
  );
}
