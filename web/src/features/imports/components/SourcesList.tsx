import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ClipboardPaste, Download, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatBytes } from "@/features/media/format";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { formatDateTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { downloadImportSource, type ImportSource } from "../api";

/** SourcesList names current originals and requests their short-lived download links until retention removes them. */
export function SourcesList({
  importId,
  sources,
  pendingUploads,
  removed = false,
}: Readonly<{
  importId: string;
  sources: readonly ImportSource[];
  pendingUploads: number;
  removed?: boolean;
}>) {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const client = useQueryClient();

  const download = async (source: ImportSource) => {
    setBusy(source.id);
    setError(null);
    try {
      const link = await downloadImportSource(importId, source.id);
      window.location.assign(link.url);
    } catch (cause) {
      setError(failureMessage(cause, t("imports.sources.downloadFailed")));
      if (cause instanceof ApiError && cause.code === "IMPORT_FILES_REMOVED")
        void client.invalidateQueries({ queryKey: ["word-import", importId] });
    } finally {
      setBusy(null);
    }
  };

  if (sources.length === 0 && pendingUploads === 0)
    return <p className="text-muted-foreground text-sm">{t("imports.noSources")}</p>;
  return (
    <div className="space-y-2">
      <ul className="divide-y rounded-md border">
        {sources.map((source) => (
          <li key={source.id} className="flex flex-wrap items-center gap-3 px-3 py-2.5">
            {source.role === "exam" && source.format === "text" ? (
              <ClipboardPaste
                className="text-muted-foreground size-4 shrink-0"
                aria-hidden="true"
              />
            ) : (
              <FileText
                className="text-muted-foreground size-4 shrink-0"
                aria-hidden="true"
              />
            )}
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium break-all">
                {source.role === "exam" && source.format === "text"
                  ? t("imports.detail.pastedText")
                  : source.filename}
              </p>
              <p className="text-muted-foreground text-xs">
                {t("imports.sources.meta", {
                  role: t(`imports.role.${source.role}`),
                  size: formatBytes(source.bytes),
                  at: formatDateTime(source.createdAt, locale),
                })}
              </p>
            </div>
            {removed ? null : (
              <Button
                variant="ghost"
                size="xs"
                disabled={busy !== null}
                aria-label={t("imports.sources.downloadNamed", {
                  name:
                    source.role === "exam" && source.format === "text"
                      ? t("imports.detail.pastedText")
                      : source.filename,
                })}
                onClick={() => void download(source)}
              >
                <Download aria-hidden="true" />
                {t("imports.sources.download")}
              </Button>
            )}
          </li>
        ))}
      </ul>
      {pendingUploads > 0 ? (
        <p className="text-muted-foreground text-xs">
          {t("imports.pendingUploads", { count: pendingUploads })}
        </p>
      ) : null}
      {error === null ? null : (
        <p role="alert" className="text-destructive text-xs">
          {error}
        </p>
      )}
    </div>
  );
}
