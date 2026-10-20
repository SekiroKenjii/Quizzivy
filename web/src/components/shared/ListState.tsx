import type { ReactNode, Ref } from "react";
import type { LucideIcon } from "lucide-react";
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
      className="space-y-2 focus-within:[&_[data-slot=skeleton]]:[animation-play-state:paused]! hover:[&_[data-slot=skeleton]]:[animation-play-state:paused]!"
    >
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className={i === rows - 1 ? "h-9 w-72" : "h-9 w-full"} />
      ))}
    </div>
  );
}

const EMPTY_TONES = {
  neutral: "bg-muted text-fg",
  muted: "bg-muted text-muted-fg",
  success: "bg-success-soft text-success-ink",
  warning: "bg-warning-soft text-warning-ink",
} as const;

/** EmptyStateTone names the colours of an EmptyState's icon tile. */
export type EmptyStateTone = keyof typeof EMPTY_TONES;

/**
 * EmptyState is the dashed card of an empty list: one sentence and at most one
 * action. With an `icon` it draws the deck's tile (44px, coloured by `tone`)
 * above a 16/600 title, which `titleRef` can take focus on; without one it is
 * the plain card every earlier caller draws.
 */
export function EmptyState({
  children,
  hint,
  action,
  icon: Icon,
  tone = "muted",
  titleRef,
}: Readonly<{
  children: string;
  hint?: string;
  action?: ReactNode;
  icon?: LucideIcon;
  tone?: EmptyStateTone;
  titleRef?: Ref<HTMLHeadingElement>;
}>) {
  if (Icon !== undefined)
    return (
      <div className="bg-card flex flex-col items-center gap-2.5 rounded-xl border border-dashed px-4.5 py-7 text-center min-[768px]:px-6 min-[768px]:py-10">
        <span
          className={`grid size-11 place-items-center rounded-xl ${EMPTY_TONES[tone]}`}
        >
          <Icon aria-hidden="true" className="size-5.5" />
        </span>
        <h2
          ref={titleRef}
          tabIndex={-1}
          className="mt-1 text-[16px] font-semibold tracking-[-0.01em] outline-none"
        >
          {children}
        </h2>
        {hint === undefined ? null : (
          <p className="text-muted-fg max-w-[46ch] text-[14px] leading-normal text-pretty">
            {hint}
          </p>
        )}
        {action === undefined ? null : <div className="mt-1.5">{action}</div>}
      </div>
    );
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
    <div role="alert" className="@container/load-error rounded-lg border p-5">
      <p className="text-sm font-medium">{children}</p>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
        {requestId === undefined ? null : (
          <div className="flex max-w-full min-w-0 flex-wrap items-center gap-x-2 gap-y-1.5">
            <span className="text-muted-foreground shrink-0 text-xs whitespace-nowrap">
              {t("common.requestId")}
            </span>
            <code className="order-last min-w-0 basis-full rounded-sm border px-1.5 py-0.5 font-mono text-xs break-all @md/load-error:order-none @md/load-error:basis-auto">
              {requestId}
            </code>
            <Button
              variant="ghost"
              size="xs"
              className="shrink-0"
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
