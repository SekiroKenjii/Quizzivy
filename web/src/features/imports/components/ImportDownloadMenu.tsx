import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { DropdownMenu as Primitive } from "radix-ui";
import { Download } from "lucide-react";
import { RowMenu } from "@/components/shared/RowMenu";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { downloadImportSource, type ImportSource, type WordImport } from "../api";

/** ImportDownloadMenu downloads only completed originals and respects retention removal. */
export function ImportDownloadMenu({ item }: Readonly<{ item: WordImport }>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [removed, setRemoved] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const download = async (source: ImportSource) => {
    setBusy(true);
    setError(null);
    try {
      const link = await downloadImportSource(item.id, source.id);
      window.location.assign(link.url);
    } catch (cause) {
      setError(
        cause instanceof ApiError && cause.code === "UNKNOWN"
          ? t("imports.sources.downloadFailed")
          : failureMessage(cause, t("imports.sources.downloadFailed")),
      );
      if (cause instanceof ApiError && cause.code === "IMPORT_FILES_REMOVED") {
        setRemoved(true);
        void client.invalidateQueries({ queryKey: ["word-imports"] });
      }
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <RowMenu label={t("imports.history.actionsNamed", { title: item.title })}>
        <Primitive.Group>
          {item.sources.length === 0 ? (
            <DropdownMenuItem disabled>{t("imports.noSources")}</DropdownMenuItem>
          ) : (
            item.sources.map((source) => (
              <DropdownMenuItem
                key={source.id}
                className="min-w-0 break-all whitespace-normal"
                disabled={busy || removed || item.filesRemovedAt !== undefined}
                onSelect={() => void download(source)}
              >
                <Download aria-hidden="true" />
                {t("imports.sources.downloadNamed", { name: source.filename })}
              </DropdownMenuItem>
            ))
          )}
        </Primitive.Group>
      </RowMenu>
      {error === null ? null : (
        <p role="alert" className="text-danger-ink text-xs break-words">
          {error}
        </p>
      )}
    </>
  );
}
