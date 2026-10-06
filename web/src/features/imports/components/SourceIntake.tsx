import { useId, useState, type ReactNode } from "react";
import { Link } from "react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { formatBytes } from "@/features/media/format";
import { getWordImportLimits, type WordImport, type ImportRetention } from "../api";
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
  deck = false,
  enabled = true,
  fileMode = true,
  retention,
  privacy,
  onChanged,
  onStarted,
}: Readonly<{
  existing: WordImport | null;
  replacing?: boolean;
  deck?: boolean;
  enabled?: boolean;
  fileMode?: boolean;
  retention?: ImportRetention | undefined;
  privacy?: ReactNode;
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
  const canStart = enabled && intake.ready && (!askTitle || title.trim() !== "");

  const files = (
    <>
      <LimitsLine
        pending={limits.isPending}
        failed={limits.isError}
        maxBytes={limits.data?.maxBytes}
        formats={limits.data?.formats}
        onRetry={() => void limits.refetch()}
      />
      <div className="flex flex-col gap-5">
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
    </>
  );

  return (
    <form
      className="flex flex-col gap-6"
      onSubmit={(event) => {
        event.preventDefault();
        if (canStart) void intake.start(title.trim());
      }}
    >
      <div hidden={!fileMode}>
        {deck ? (
          <Card>
            <CardHeader>
              <CardTitle>{t("imports.newHeading")}</CardTitle>
              <CardDescription>{t("imports.newHint")}</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-5">{files}</CardContent>
          </Card>
        ) : (
          files
        )}
      </div>
      {askTitle ? (
        <div className="flex flex-col gap-1.5">
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
      <Recognition deck={deck} replacing={replacing} />
      {deck ? (
        <>
          {privacy}
          {retention === undefined ? null : (
            <p className="text-muted-fg text-xs leading-relaxed">
              {t("imports.retention.policy", {
                afterCommit: retention.afterCommitDays,
                afterCancel: retention.afterCancelDays,
                idle: retention.idleDays,
              })}
            </p>
          )}
        </>
      ) : null}
      {intake.error === null ? null : (
        <p role="alert" className="text-destructive text-sm">
          {intake.error}
        </p>
      )}
      <IntakeFooter
        deck={deck}
        fileMode={fileMode}
        canStart={canStart}
        busy={intake.busy}
        replacing={replacing}
      />
    </form>
  );
}

function Recognition({
  deck,
  replacing,
}: Readonly<{ deck: boolean; replacing: boolean }>) {
  const { t } = useTranslation();
  if (replacing) return null;
  if (!deck)
    return (
      <p className="text-muted-foreground text-xs leading-relaxed">
        {t("imports.upload.recognitionNote")}
      </p>
    );
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("imports.upload.recognitionHeading")}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-sm font-medium">{t("imports.upload.automatic")}</p>
        <p className="text-muted-fg text-xs leading-relaxed">
          {t("imports.upload.recognitionNote")}
        </p>
      </CardContent>
    </Card>
  );
}

function IntakeFooter({
  deck,
  fileMode,
  canStart,
  busy,
  replacing,
}: Readonly<{
  deck: boolean;
  fileMode: boolean;
  canStart: boolean;
  busy: boolean;
  replacing: boolean;
}>) {
  const { t } = useTranslation();
  let hint = t("imports.upload.chooseHint");
  if (busy) hint = t("imports.upload.starting");
  else if (!fileMode) hint = t("imports.upload.pasteDeferredHint");
  else if (canStart) hint = t("imports.upload.readyHint");
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-3",
        deck && "bg-background sticky bottom-0 justify-end border-t py-3",
      )}
    >
      {deck ? (
        <>
          <span className="text-muted-fg min-w-0 flex-[1_1_200px] text-xs">{hint}</span>
          <Button asChild type="button" variant="outline">
            <Link to="/teacher/imports">{t("common.cancel")}</Link>
          </Button>
        </>
      ) : null}
      <Button type="submit" disabled={!canStart}>
        {submitLabel(busy, replacing, t)}
      </Button>
      {busy ? (
        <span role="status" className="text-muted-foreground text-xs">
          {t("imports.upload.keepOpen")}
        </span>
      ) : null}
    </div>
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
