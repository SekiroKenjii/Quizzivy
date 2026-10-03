import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { HOME_ITEM, HOME_LIST, HOME_ROW } from "./homeStyles";

const ROWS = ["first", "second", "third"] as const;

/**
 * HomeSkeleton stands in for Home's lists while they load: the resume card
 * and three rows, in grey, at the sizes the real ones take.
 */
export function HomeSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="flex flex-col gap-6"
    >
      <div className="flex flex-wrap items-center gap-4.5 rounded-2xl border p-5">
        <div className="flex flex-[1_1_280px] flex-col gap-2.5">
          <Skeleton className="h-6 w-40 rounded-full" />
          <Skeleton className="h-5 w-[70%] rounded-sm" />
          <Skeleton className="h-3.5 w-1/2 rounded-sm" />
          <Skeleton className="h-1.5 w-[60%] rounded-sm" />
        </div>
        <Skeleton className="h-11.5 flex-[1_1_100%] rounded-lg min-[768px]:w-40 min-[768px]:flex-none" />
      </div>
      <div className={HOME_LIST}>
        {ROWS.map((row) => (
          <div key={row} className={HOME_ITEM}>
            <div className={HOME_ROW}>
              <Skeleton className="rounded-ctl h-12 w-11 flex-none" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-3.5 w-[55%] rounded-sm" />
                <Skeleton className="h-3 w-[35%] rounded-sm" />
              </div>
              <Skeleton className="h-6 w-20 rounded-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
