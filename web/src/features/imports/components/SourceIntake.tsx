import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { formatBytes } from "@/features/media/format";
import { getWordImportLimits, type WordImport } from "../api";
import { SOURCE_ROLES, useSourceIntake } from "../useSourceIntake";
import { FileSlot } from "./FileSlot";

const TITLE_MAX = 200;

function titleFromFilename(name: string): string {
  return name
    .replace(/\.(docx?|pdf)$/i, "")
    .trim()
    .slice(0, TITLE_MAX);
}

/**
 * SourceIntake is the upload form for a new import, one awaiting its sources,
 * and, with `replacing`, a failed one whose files are replaced: the limits,
 * the exam and optional key, and one action that uploads what changed and
 * queues processing.
 */
export function SourceIntake({
  existing,
  replacing = false,
  onChanged,
  onStarted,
}: Readonly<{
  existing: WordImport | null;
  replacing?: boolean;
  onChanged?: ((next: WordImport) => void) | undefined;
  onStarted: (started: WordImport) => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const limits = useQuery({
    queryKey: ["word-import-limits"],
    queryFn: ({ signal }) => getWordImportLimits(signal),
    staleTime: 5 * 60_000,
  });
  const intake = useSourceIntake({
    existing,
    limits: limits.data,
    replacing,
    onChanged,
    onStarted,
  });
  const [typedTitle, setTypedTitle] = useState<string | null>(null);
  const examName = intake.slots.exam.file?.name ?? intake.slots.exam.received?.filename;
  const title =
    typedTitle ?? (examName === undefined ? "" : titleFromFilename(examName));
  const askTitle = existing === null;
  const titleLocked = intake.importId !== null;
  const accept = (limits.data?.formats ?? ["docx"])
    .map((format) => `.${format}`)
    .join(",");
  const canStart = intake.ready && (!askTitle || title.trim() !== "");

  return (
    <form
      className="space-y-6"
      onSubmit={(event) => {
        event.preventDefault();
        if (canStart) void intake.start(title.trim());
      }}
    >
      <LimitsLine
        pending={limits.isPending}
        failed={limits.isError}
        maxBytes={limits.data?.maxBytes}
        formats={limits.data?.formats}
        onRetry={() => void limits.refetch()}
      />
      <div className="space-y-5">
        {SOURCE_ROLES.map((role) => (
          <FileSlot
            key={role}
            role={role}
            slot={intake.slots[role]}
            accept={accept}
            required={role === "exam"}
            disabled={intake.busy}
            onChoose={(file) => intake.choose(role, file)}
            onRemove={() => intake.remove(role)}
          />
        ))}
      </div>
      {askTitle ? (
        <div className="space-y-1.5">
          <Label htmlFor={titleId}>{t("imports.upload.titleLabel")}</Label>
          <Input
            id={titleId}
            value={title}
            maxLength={TITLE_MAX}
            disabled={titleLocked || intake.busy}
            placeholder={t("imports.upload.titlePlaceholder")}
            onChange={(event) => setTypedTitle(event.target.value)}
          />
          <p className="text-muted-foreground text-xs">
            {titleLocked
              ? t("imports.upload.titleLocked")
              : t("imports.upload.titleHint")}
          </p>
        </div>
      ) : null}
      {replacing ? null : (
        <p className="text-muted-foreground text-xs leading-relaxed">
          {t("imports.upload.recognitionNote")}
        </p>
      )}
      {intake.error === null ? null : (
        <p role="alert" className="text-destructive text-sm">
          {intake.error}
        </p>
      )}
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={!canStart}>
          {submitLabel(intake.busy, replacing, t)}
        </Button>
        {intake.busy ? (
          <span role="status" className="text-muted-foreground text-xs">
            {t("imports.upload.keepOpen")}
          </span>
        ) : null}
      </div>
    </form>
  );
}

function submitLabel(busy: boolean, replacing: boolean, t: TFunction): string {
  if (busy) return t("imports.upload.starting");
  return replacing ? t("imports.upload.replaceAndStart") : t("imports.upload.start");
}

function LimitsLine({
  pending,
  failed,
  maxBytes,
  formats,
  onRetry,
}: Readonly<{
  pending: boolean;
  failed: boolean;
  maxBytes: number | undefined;
  formats: readonly string[] | undefined;
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  if (pending)
    return (
      <p role="status" className="text-muted-foreground text-sm">
        {t("imports.upload.limitsLoading")}
      </p>
    );
  if (failed || maxBytes === undefined || formats === undefined)
    return (
      <p role="alert" className="flex flex-wrap items-center gap-2 text-sm">
        {t("imports.upload.limitsFailed")}
        <Button type="button" variant="outline" size="xs" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      </p>
    );
  return (
    <p className="bg-muted/40 rounded-md p-3 text-sm">
      {t("imports.upload.limits", {
        formats: formats.map((format) => `.${format}`).join(", "),
        size: formatBytes(maxBytes),
      })}
    </p>
  );
}
