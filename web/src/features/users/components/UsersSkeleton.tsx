import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";

const ROWS = 8;

/** UsersSkeleton stands for the table while its first page is on its way. */
export function UsersSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="bg-card shadow-card overflow-hidden rounded-xl border"
    >
      <div className="bg-muted h-10" />
      {Array.from({ length: ROWS }, (_, index) => (
        <div key={index} className="flex h-14.5 items-center gap-3 border-t px-4">
          <Skeleton className="size-8 flex-none rounded-full" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-20 flex-none" />
        </div>
      ))}
    </div>
  );
}
