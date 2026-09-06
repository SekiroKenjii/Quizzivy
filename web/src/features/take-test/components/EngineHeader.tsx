import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { Clock } from "./Clock";
import { SaveState } from "./SaveState";
import { useTakeTestStore } from "../store";

/**
 * The engine's header, one height in every view so the timer never moves
 * (S-05, S-15). On a phone: the leading control, the counter, the clock. From
 * 1024px the row has room for everything S-08 puts there -- the test's title,
 * the save state, the strike count and the clock -- and the save strip goes.
 */
export function EngineHeader({
  wide,
  leading,
  counter,
  progress,
  status = null,
  live = true,
}: Readonly<{
  wide: boolean;
  /** "Thoát" on the paper, "Quay lại bài" on the review, nothing once submitted. */
  leading: ReactNode;
  counter?: { n: number; total: number };
  /** 0..1, or null for a view with no position in the paper. */
  progress: number | null;
  /** The strike indicator, when the assignment counts focus loss. */
  status?: ReactNode;
  /** False after submission: the title stays, the clock and the save state go. */
  live?: boolean;
}>) {
  const { t } = useTranslation();
  const title = useTakeTestStore((s) => s.testTitle);

  return (
    <header className="border-b">
      <div
        className={cn(
          "flex h-12 items-center gap-3",
          wide ? "px-5" : "mx-auto w-full max-w-[720px] px-4",
        )}
      >
        {leading}
        {wide ? (
          <>
            <span className="text-muted-foreground min-w-0 truncate text-xs">
              {title}
            </span>
            {live && (
              <>
                <Badge variant="outline" className="ml-auto shrink-0">
                  <SaveState />
                </Badge>
                {status !== null && (
                  <span className="text-muted-foreground shrink-0 text-xs">
                    {status}
                  </span>
                )}
                <span className="bg-border mx-1 h-4 w-px shrink-0" aria-hidden="true" />
                <Clock />
              </>
            )}
          </>
        ) : (
          <>
            {counter !== undefined && (
              <span className="text-muted-foreground ml-auto text-xs tabular-nums">
                {t("takeTest.questionCounter", counter)}
              </span>
            )}
            {live && (
              <span className={counter === undefined ? "ml-auto" : undefined}>
                <Clock />
              </span>
            )}
          </>
        )}
      </div>
      {progress !== null && (
        <div className="bg-secondary h-1">
          <div
            className="bg-primary h-full transition-[width]"
            style={{ width: `${(progress * 100).toFixed(2)}%` }}
          />
        </div>
      )}
    </header>
  );
}
