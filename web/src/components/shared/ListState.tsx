import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { ApiError } from "@/lib/api/errors";

/** F-08's three list states: skeleton rows, one sentence + one action, and the route error card. */

export function ListSkeleton({ rows = 5 }: Readonly<{ rows?: number }>) {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="space-y-2"
    >
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className={i === rows - 1 ? "h-9 w-72" : "h-9 w-full"} />
      ))}
    </div>
  );
}

export function EmptyState({
  children,
  hint,
  action,
}: Readonly<{
  children: string;
  hint?: string;
  action?: ReactNode;
}>) {
  return (
    <div className="rounded-lg border border-dashed p-8 text-center">
      <p className="text-sm">{children}</p>
      {hint === undefined ? null : (
        <p className="text-muted-foreground mt-1.5 text-xs leading-relaxed">{hint}</p>
      )}
      {action === undefined ? null : <div className="mt-3">{action}</div>}
    </div>
  );
}

export function LoadError({
  children,
  error,
  onRetry,
}: Readonly<{
  children: string;
  error: unknown;
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  const requestId = error instanceof ApiError ? error.requestId : undefined;
  return (
    <div role="alert" className="rounded-lg border p-5">
      <p className="text-sm font-medium">{children}</p>
      <div className="mt-4 flex items-center justify-between gap-3">
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
        {requestId === undefined ? null : (
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {t("common.requestId")}
            </span>
            <code className="rounded-sm border px-1.5 py-0.5 font-mono text-xs">
              {requestId}
            </code>
            <Button
              variant="ghost"
              size="xs"
              onClick={() => {
                void navigator.clipboard.writeText(requestId);
                toast(t("common.copied"));
              }}
            >
              {t("common.copy")}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

interface QueryLike<T> {
  readonly isPending: boolean;
  readonly isError: boolean;
  readonly error: unknown;
  readonly data: T | undefined;
  readonly refetch: () => unknown;
}

/**
 * F-08's states ahead of the content, written once: the skeleton while the
 * first read is out, the error card with its retry, then the content -- so no
 * screen nests one ternary inside another to get there.
 */
export function QueryStates<T>({
  query,
  skeleton,
  failed,
  className,
  children,
}: Readonly<{
  query: QueryLike<T>;
  skeleton: ReactNode;
  failed: string;
  /** Wraps the skeleton and the error, for a card whose body carries its own padding. */
  className?: string;
  children: (data: T) => ReactNode;
}>) {
  const wrap = (node: ReactNode) =>
    className === undefined ? node : <div className={className}>{node}</div>;
  if (query.isPending) return wrap(skeleton);
  if (query.isError) {
    return wrap(
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {failed}
      </LoadError>,
    );
  }
  if (query.data === undefined) return null;
  return children(query.data);
}
