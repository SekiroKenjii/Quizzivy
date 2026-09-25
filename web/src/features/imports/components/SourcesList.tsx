import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Download, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatBytes } from "@/features/media/format";
import { failureMessage } from "@/lib/api/errors";
import { formatDateTime } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { downloadImportSource, type ImportSource } from "../api";

/** SourcesList names an import's current original files and hands out short-lived download links on request. */
export function SourcesList({
  importId,
  sources,
  pendingUploads,
}: Readonly<{
  importId: string;
  sources: readonly ImportSource[];
  pendingUploads: number;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const download = async (source: ImportSource) => {
    setBusy(source.id);
    setError(null);
    try {
      const link = await downloadImportSource(importId, source.id);
      window.location.assign(link.url);
    } catch (cause) {
      setError(failureMessage(cause, t("imports.sources.downloadFailed")));
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
            <FileText
              className="text-muted-foreground size-4 shrink-0"
              aria-hidden="true"
            />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium break-all">{source.filename}</p>
              <p className="text-muted-foreground text-xs">
                {t("imports.sources.meta", {
                  role: t(`imports.role.${source.role}`),
                  size: formatBytes(source.bytes),
                  at: formatDateTime(source.createdAt, locale),
                })}
              </p>
            </div>
            <Button
              variant="ghost"
              size="xs"
              disabled={busy !== null}
              aria-label={t("imports.sources.downloadNamed", { name: source.filename })}
              onClick={() => void download(source)}
            >
              <Download aria-hidden="true" />
              {t("imports.sources.download")}
            </Button>
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
