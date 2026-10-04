import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

const TONE = {
  success: "bg-success-soft text-success-ink",
  warning: "bg-warning-soft text-warning-ink",
  muted: "bg-muted text-fg",
} as const;

/**
 * EndState is the centred column the engine shows once the paper is out of
 * the student's hands: a 48px tile with an icon, one heading, and whatever the
 * caller puts beneath. The deck goes straight from Submit to the result and
 * draws no such screen, so this is built from its parts: the tile, the 20px
 * heading and the 480px column of its one centred page.
 */
export function EndState({
  icon: Icon,
  tone,
  title,
  children,
}: Readonly<{
  icon: LucideIcon;
  tone: keyof typeof TONE;
  title: string;
  children: ReactNode;
}>) {
  return (
    <main className="min-h-0 flex-1 overflow-y-auto px-4 pt-[10vh] pb-7 min-[768px]:px-6">
      <div className="mx-auto flex w-full max-w-120 flex-col items-center gap-3 text-center">
        <span
          aria-hidden="true"
          className={cn("grid size-12 place-items-center rounded-xl", TONE[tone])}
        >
          <Icon className="size-[22px]" />
        </span>
        <h1 className="text-xl leading-normal font-semibold text-balance">{title}</h1>
        {children}
      </div>
    </main>
  );
}
