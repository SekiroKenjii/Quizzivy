import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

const TONE = {
  warning: "bg-warning-soft text-warning-ink",
  muted: "bg-muted text-muted-fg",
} as const;

/**
 * NoticeBar is the one-line bar under the engine's header for something the
 * student has to know while the paper stays on screen: fullscreen was left,
 * or the paper is locked. The deck draws neither, so the bar is built from
 * its parts: an icon and a sentence in a soft tone, with the header's side
 * padding, and at most one small bordered button at the far end.
 */
export function NoticeBar({
  icon: Icon,
  tone = "warning",
  action,
  children,
}: Readonly<{
  icon: LucideIcon;
  tone?: keyof typeof TONE;
  action?: ReactNode;
  children: ReactNode;
}>) {
  return (
    <div
      data-slot="notice-bar"
      className={cn(
        "flex flex-none items-center gap-2.5 border-b px-3.5 py-2 min-[768px]:px-6",
        TONE[tone],
      )}
    >
      <Icon aria-hidden="true" className="size-4 flex-none" />
      <p className="min-w-0 flex-1 text-sm leading-normal text-pretty">{children}</p>
      {action}
    </div>
  );
}
