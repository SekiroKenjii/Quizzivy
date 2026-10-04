import type { ReactNode } from "react";
import { PAPER_SURFACE } from "@/components/shared/content/paperSurface";
import { useLargerTestText } from "@/lib/testText";
import { cn } from "@/lib/utils";

const MATERIALS = [
  "leading-[1.75] text-pretty",
  "[&>div]:gap-3.5 [&_section]:gap-3.5",
  "[&>div>div]:text-muted-fg [&>div>div]:text-base",
  "[&_section>h4]:text-title [&_section>h4]:font-semibold",
  "[&_section>div]:leading-[1.75]",
  "[&_.semantic-content>*+*]:mt-3.5!",
].join(" ");

/**
 * PassageBody is the passage pane's column, as the deck draws it: at most
 * 640px wide, the part as an eyebrow, the title at 22px, and the materials at
 * 16px with a 1.75 line, or 18px under "Larger text in tests". Images and
 * rich tables inside it keep the paper surface in dark mode.
 */
export function PassageBody({
  eyebrow,
  title,
  heading: Heading = "h2",
  children,
}: Readonly<{
  eyebrow?: string | undefined;
  title: string;
  heading?: "h2" | "h3";
  children: ReactNode;
}>) {
  const larger = useLargerTestText();
  return (
    <div
      className={cn("mx-auto flex w-full max-w-160 flex-col gap-3.5", PAPER_SURFACE)}
    >
      {eyebrow ? (
        <p className="text-muted-fg text-meta leading-normal font-semibold tracking-[0.02em] uppercase">
          {eyebrow}
        </p>
      ) : null}
      <Heading className="text-stat leading-[1.3] font-semibold tracking-[-0.01em]">
        {title}
      </Heading>
      <div className={cn(larger ? "text-stat-sm" : "text-title", MATERIALS)}>
        {children}
      </div>
    </div>
  );
}
