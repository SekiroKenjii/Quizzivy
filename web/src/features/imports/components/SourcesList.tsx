import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ClipboardPaste, Download, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatBytes } from "@/features/media/format";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { formatDateTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import { downloadImportSource, type ImportSource } from "../api";

/**
 * SourcePill is the state a source's card shows beside its name: waiting,
 * being read, read, or the file processing stopped at.
 */
export type SourcePill = "waiting" | "reading" | "read" | "failed";

const PILL_TONES: Record<SourcePill, string> = {
  waiting: "bg-muted text-muted-fg",
  reading: "bg-info-soft text-info-ink",
  read: "bg-success-soft text-success-ink",
  failed: "bg-danger-soft text-danger-ink",
};

/**
 * SourcesList is the Sources pane of an import that has started: one card per
 * current original, with its kind and size, the state `pillOf` gives it, and
 * a short-lived download link until retention removes the files.
 */
export function SourcesList({
  importId,
  sources,
  pendingUploads,
  removed = false,
  pillOf,
}: Readonly<{
  importId: string;
  sources: readonly ImportSource[];
  pendingUploads: number;
  removed?: boolean;
  pillOf?: ((source: ImportSource) => SourcePill | null) | undefined;
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
    <div className="flex flex-col gap-3">
      <ul className="flex flex-col gap-3">
        {sources.map((source) => {
          const pill = pillOf?.(source) ?? null;
          return (
            <li
              key={source.id}
              className="flex flex-wrap items-center gap-3 rounded-[10px] border px-3.5 py-3"
            >
              <span className="bg-muted grid size-[34px] shrink-0 place-items-center rounded-[8px]">
                {source.role === "exam" && source.format === "text" ? (
                  <ClipboardPaste className="size-4" aria-hidden="true" />
                ) : (
                  <FileText className="size-4" aria-hidden="true" />
                )}
              </span>
              <div className="min-w-0 flex-[1_1_180px]">
                <p className="text-ui font-medium break-all">
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
              {pill === null ? null : (
                <span
                  className={cn(
                    "inline-flex h-5.5 shrink-0 items-center rounded-full px-2 text-xs font-medium whitespace-nowrap",
                    PILL_TONES[pill],
                  )}
                >
                  {t(`imports.workspace.fileState.${pill}`)}
                </span>
              )}
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
          );
        })}
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
