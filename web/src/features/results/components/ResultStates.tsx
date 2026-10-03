import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { CircleAlert, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { ApiError } from "@/lib/api/errors";

const SHAPES = ["first", "second"] as const;
const CARD = "bg-card shadow-card rounded-2xl border p-5.5";

/**
 * ResultSkeleton stands in for the result while it loads: the summary card
 * with its ring, the answers' heading and two answers, in grey, at the sizes
 * the real ones take.
 */
export function ResultSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="flex flex-col gap-4.5"
    >
      <div className={`${CARD} flex flex-wrap items-center gap-5.5`}>
        <Skeleton className="size-28 flex-none rounded-full" />
        <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-2.5">
          <Skeleton className="h-3.5 w-[45%] rounded-sm" />
          <Skeleton className="h-5 w-[70%] rounded-sm" />
          <Skeleton className="h-3.5 w-[85%] rounded-sm" />
        </div>
      </div>
      <div className="flex flex-col gap-2.5">
        <Skeleton className="h-6 w-32 rounded-sm" />
        {SHAPES.map((shape) => (
          <div key={shape} className="flex gap-3 rounded-xl border px-4 py-3.5">
            <Skeleton className="size-6.5 flex-none rounded-full" />
            <div className="flex flex-1 flex-col gap-2">
              <Skeleton className="h-4 w-[80%] rounded-sm" />
              <Skeleton className="h-3.5 w-[45%] rounded-sm" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function Away({ sentence, detail }: Readonly<{ sentence: string; detail?: string }>) {
  const { t } = useTranslation();
  return (
    <div className={`${CARD} flex flex-col items-center gap-1 text-center`}>
      <p className="text-base">{sentence}</p>
      {detail !== undefined && <p className="text-muted-fg text-sm">{detail}</p>}
      <Button asChild className="mt-3 w-full">
        <Link to="/app">{t("takeTest.backHome")}</Link>
      </Button>
    </div>
  );
}

/**
 * ResultFailure is what the page shows when the result could not be read. An
 * attempt that is not submitted, or was voided, says so with the server's
 * sentence and a way Home. A result that is not the student's, or does not
 * exist, says it was not found, with a way Home and no retry. Anything else
 * says the paper is safe and offers to try again, with the request id to
 * quote. Where the card is too narrow for the label, the id and Copy on one
 * line, the id takes a line of its own under the other two, as it does in
 * LoadError.
 */
export function ResultFailure({
  error,
  onRetry,
}: Readonly<{ error: unknown; onRetry: () => void }>) {
  const { t } = useTranslation();
  if (error instanceof ApiError && error.status === 409)
    return <Away sentence={t("result.notReady")} detail={error.message} />;
  if (error instanceof ApiError && (error.status === 403 || error.status === 404))
    return <Away sentence={t("result.notFound")} />;
  const requestId = error instanceof ApiError ? error.requestId : undefined;
  return (
    <div role="alert" className={`${CARD} flex items-start gap-2.5`}>
      <CircleAlert
        aria-hidden="true"
        className="text-muted-fg mt-0.5 size-4.5 flex-none"
      />
      <div className="@container/load-error min-w-0 flex-1 text-base">
        <p className="font-medium">{t("result.loadFailed")}</p>
        <p className="text-muted-fg">{t("result.loadFailedBody")}</p>
        <div className="mt-3">
          <Button size="sm" onClick={onRetry}>
            <RefreshCw aria-hidden="true" />
            {t("common.retry")}
          </Button>
        </div>
        {requestId !== undefined && (
          <div className="mt-3 flex max-w-full min-w-0 flex-wrap items-center gap-x-2 gap-y-1.5">
            <span className="text-muted-fg shrink-0 text-xs whitespace-nowrap">
              {t("common.requestId")}
            </span>
            <code className="order-last min-w-0 basis-full rounded-sm border px-1.5 py-0.5 font-mono text-xs break-all @md/load-error:order-none @md/load-error:basis-auto">
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
