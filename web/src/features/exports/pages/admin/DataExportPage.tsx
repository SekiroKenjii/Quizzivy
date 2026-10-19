import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { Download, FileX2, Hourglass, TriangleAlert } from "lucide-react";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { Callout } from "@/components/shared/Callout";
import { EmptyState } from "@/components/shared/ListState";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  dataExportKey,
  dataExportQuery,
  downloadDataExport,
  EXPORT_POLL_MS,
  MAX_DOWNLOADS,
  type DataExport,
} from "@/features/exports/api";
import { formatStorage } from "@/features/media/format";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { formatDateTime } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { PageHead } from "@/layouts/shell/PageHead";

const BADGES = {
  queued: "secondary",
  running: "primary",
  ready: "success",
  failed: "danger",
  expired: "outline",
} as const;

function Row({ label, children }: Readonly<{ label: string; children: string }>) {
  return (
    <div className="flex flex-wrap items-baseline justify-between gap-x-3 px-4.5 py-3">
      <dt className="text-muted-fg text-ui">{label}</dt>
      <dd className="text-ui font-medium">{children}</dd>
    </div>
  );
}

function ReadyActions({ data }: Readonly<{ data: DataExport }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const client = useQueryClient();
  const [pending, setPending] = useState(false);
  const busy = useRef(false);
  const [error, setError] = useState<string | null>(null);

  async function download() {
    if (busy.current) return;
    busy.current = true;
    setPending(true);
    setError(null);
    try {
      const { url } = await downloadDataExport(data.id);
      const link = document.createElement("a");
      link.href = url;
      link.rel = "noopener";
      document.body.append(link);
      link.click();
      link.remove();
      await client.invalidateQueries({ queryKey: dataExportKey(data.id) });
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "EXPORT_UNAVAILABLE") {
        await client.invalidateQueries({ queryKey: dataExportKey(data.id) });
      }
      setError(failureMessage(cause, t("api.failed")));
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  const taken = data.downloads;
  return (
    <div className="flex flex-col gap-2 px-4.5 py-3">
      {taken > 0 ? (
        <p className="text-muted-fg text-ui">
          {data.downloadedAt === null
            ? t("dataExport.downloadCount", { count: taken, max: MAX_DOWNLOADS })
            : t("dataExport.downloadedAt", {
                time: formatDateTime(data.downloadedAt, locale),
                count: taken,
                max: MAX_DOWNLOADS,
              })}
        </p>
      ) : null}
      {taken < MAX_DOWNLOADS ? (
        <div>
          <Button
            type="button"
            aria-disabled={pending || undefined}
            className="aria-disabled:bg-primary/50 aria-disabled:hover:bg-primary/50 h-8.5"
            onClick={() => void download()}
          >
            <Download aria-hidden="true" />
            {t("dataExport.download")}
          </Button>
        </div>
      ) : (
        <p className="text-muted-fg text-ui">{t("dataExport.exhausted")}</p>
      )}
      {error === null ? null : (
        <p role="alert" className="text-danger-ink text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

function Details({ data }: Readonly<{ data: DataExport }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <SettingsCard
      title={t("dataExport.cardTitle")}
      action={
        <Badge variant={BADGES[data.status]}>
          {t(`dataExport.status.${data.status}`)}
        </Badge>
      }
    >
      <dl className="divide-y">
        <Row label={t("dataExport.requested")}>
          {formatDateTime(data.createdAt, locale)}
        </Row>
        <Row label={t("dataExport.contents")}>
          {data.includeAnswers
            ? t("dataExport.withAnswers")
            : t("dataExport.withoutAnswers")}
        </Row>
        {data.bytes === null ? null : (
          <Row label={t("dataExport.size")}>{formatStorage(data.bytes)}</Row>
        )}
        {data.status === "ready" && data.expiresAt !== null ? (
          <Row label={t("dataExport.expires")}>
            {formatDateTime(data.expiresAt, locale)}
          </Row>
        ) : null}
      </dl>
      {data.status === "ready" ? (
        <div className="border-t">
          <ReadyActions data={data} />
        </div>
      ) : null}
    </SettingsCard>
  );
}

function Waiting({ status }: Readonly<{ status: "queued" | "running" }>) {
  const { t } = useTranslation();
  return (
    <Callout tone="info" icon={Hourglass}>
      {t(`dataExport.waiting.${status}`)}
    </Callout>
  );
}

/**
 * DataExportPage is the Admin console's `exports/:id` page, where the link in the export's email
 * lands once the requester has signed in. A queued or running export is read
 * again every five seconds while somebody is there; a ready one shows its size
 * and expiry and offers the file once, then says when it was taken. A failed
 * export, an expired one and one that is not the caller's, which the server
 * answers as missing, each say so in place.
 */
export default function DataExportPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const poll = useIdlePolling(EXPORT_POLL_MS);
  const exportQuery = useQuery({
    ...dataExportQuery(id),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "queued" || status === "running" ? poll : false;
    },
  });
  const status = exportQuery.data?.status;
  useRefetchOnResume(
    () => void exportQuery.refetch(),
    status === "queued" || status === "running",
  );
  const missing =
    exportQuery.error instanceof ApiError && exportQuery.error.status === 404;
  const data = exportQuery.data;

  let body;
  if (data !== undefined) {
    body = (
      <>
        {data.status === "queued" || data.status === "running" ? (
          <Waiting status={data.status} />
        ) : null}
        {data.status === "failed" ? (
          <Callout tone="danger" icon={TriangleAlert} announce>
            {t("dataExport.failed")}
          </Callout>
        ) : null}
        {data.status === "expired" ? (
          <Callout tone="warning" icon={FileX2}>
            {t("dataExport.expiredNote")}
          </Callout>
        ) : null}
        <Details data={data} />
      </>
    );
  } else if (missing) {
    body = (
      <EmptyState icon={FileX2} hint={t("dataExport.missingHint")}>
        {t("dataExport.missing")}
      </EmptyState>
    );
  } else if (exportQuery.isError) {
    body = (
      <div className="bg-card shadow-card flex flex-wrap items-center gap-3 rounded-xl border px-4.5 py-4">
        <p role="alert" className="text-muted-fg text-ui min-w-40 flex-1">
          {failureMessage(exportQuery.error, t("dataExport.loadFailed"))}
        </p>
        <Button
          type="button"
          variant="outline"
          className="h-8.5"
          onClick={() => void exportQuery.refetch()}
        >
          {t("common.retry")}
        </Button>
      </div>
    );
  } else {
    body = (
      <div
        role="status"
        aria-label={t("common.loading")}
        className="bg-card shadow-card flex flex-col gap-3 rounded-xl border p-4.5"
      >
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4.5">
      <PageHead
        title={t("dataExport.title")}
        description={t("dataExport.hint")}
        back={{
          to: `${ADMIN_PATHS.settings}/data`,
          label: t("dataExport.back"),
        }}
      />
      {body}
    </div>
  );
}
