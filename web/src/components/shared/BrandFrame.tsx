import type { ReactNode } from "react";

import { BrandLockup } from "@/components/shared/Brand";
import { cn } from "@/lib/utils";

const FROM = {
  auth: { panel: "auth:flex", narrow: "auth:hidden", main: "auth:p-12" },
  lg: { panel: "lg:flex", narrow: "lg:hidden", main: "lg:p-12" },
} as const;

/**
 * BrandFrame is the deck's page for everything outside a console: a brand
 * panel beside a centred 380px column. The panel shows from `from` (900px on
 * the sign-in pages, 1024px on the system pages) and holds the logo, `panel`
 * in the middle and `caption` at the foot; below that width a small logo sits
 * above the column instead. The frame is a deck surface.
 */
export function BrandFrame({
  from,
  panel,
  caption,
  showPanel = true,
  children,
}: Readonly<{
  from: keyof typeof FROM;
  panel?: ReactNode;
  caption?: ReactNode;
  showPanel?: boolean;
  children: ReactNode;
}>) {
  const at = FROM[from];
  return (
    <div data-scale="deck" className="bg-bg text-fg flex min-h-svh">
      {showPanel && (
        <aside
          className={cn(
            "bg-sidebar hidden max-w-[620px] shrink-0 grow-0 basis-[44%] flex-col justify-between border-r px-12 py-10",
            at.panel,
          )}
        >
          <BrandLockup height={32} theme="auto" className="self-start" />
          {panel}
          <div className="text-muted-fg text-meta">{caption}</div>
        </aside>
      )}
      <main
        className={cn(
          "flex min-w-0 flex-1 flex-col items-center justify-center px-5 py-8",
          showPanel && at.main,
        )}
      >
        <div className={cn("mb-7", showPanel && at.narrow)}>
          <BrandLockup height={28} theme="auto" />
        </div>
        <div className="flex w-full max-w-[380px] flex-col">{children}</div>
      </main>
    </div>
  );
}
