import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Legend, SectionBlocks, type Squares } from "./Navigator";

/**
 * SectionRail is the engine's sections rail, the column at the paper's left
 * edge: every section's title and answered count over a grid of its
 * questions' squares, scrolling by itself, with the legend at its foot. The
 * page decides when it shows; it keeps the open question's square in view.
 */
export function SectionRail({ dots, current, groups, onJump }: Readonly<Squares>) {
  const { t } = useTranslation();
  const now = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    now.current?.scrollIntoView?.({ block: "nearest" });
  }, [current]);

  return (
    <aside
      aria-label={t("takeTest.sectionsRail")}
      className="bg-sidebar flex w-59 flex-none flex-col gap-4 overflow-y-auto border-r px-3.5 py-4"
    >
      <SectionBlocks
        shape="rail"
        dots={dots}
        current={current}
        groups={groups}
        currentRef={now}
        onJump={onJump}
      />
      <Legend className="mt-auto pt-2" />
    </aside>
  );
}
