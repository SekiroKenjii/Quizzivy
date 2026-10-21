import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";

/** RolesSkeleton stands for the carousel and the matrix while the roles load. */
export function RolesSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="flex flex-col gap-3"
    >
      <div className="flex gap-3 overflow-hidden">
        {[0, 1, 2].map((slot) => (
          <Skeleton key={slot} className="h-49 min-w-0 flex-1 rounded-[1rem]" />
        ))}
      </div>
      <div className="bg-card overflow-hidden rounded-xl border">
        <Skeleton className="h-11 rounded-none" />
        {Array.from({ length: 8 }, (_, row) => (
          <div key={row} className="flex items-center gap-4 border-t px-4 py-3.5">
            <Skeleton className="h-3.5 w-56 max-w-1/2" />
            <Skeleton className="ml-auto h-4.5 w-4.5 rounded-[0.3125rem]" />
            <Skeleton className="h-4.5 w-4.5 rounded-[0.3125rem]" />
          </div>
        ))}
      </div>
    </div>
  );
}
