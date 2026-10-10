import {
  useCallback,
  useId,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { Link, useBeforeUnload, useBlocker } from "react-router";
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
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { formatBytes } from "@/features/media/format";
import { commandKeyLabel } from "@/features/search/useCommandPalette";
import { dayMonth } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { getWordImportLimits, type WordImport, type ImportRetention } from "../api";
import { usePastedText, type PasteState } from "../usePastedText";
import { SOURCE_ROLES, useSourceIntake } from "../useSourceIntake";
import { FileSlot } from "./FileSlot";
import { PasteField } from "./PasteField";

const TITLE_MAX = 200;
const PASTE_MAX = 100_000;

function pasteHint(
  state: PasteState,
  questions: number,
  max: number,
  t: TFunction,
  locale: string,
): string {
  if (state === "empty") return t("imports.paste.emptyHint");
  if (state === "over")
    return t("imports.paste.overHint", {
      max: new Intl.NumberFormat(locale).format(max),
    });
  if (state === "none") return t("imports.paste.noQuestionsHint");
  return [
    t("imports.paste.foundHint", { count: questions }),
    t("imports.paste.processingHint"),
    t("imports.paste.shortcutHint", { key: commandKeyLabel() }),
  ].join(" ");
}

function footerHint(
  {
    busy,
    pasting,
    canStart,
    paste,
    questions,
    max,
  }: {
    busy: boolean;
    pasting: boolean;
    canStart: boolean;
    paste: PasteState;
    questions: number;
    max: number;
  },
  t: TFunction,
  locale: string,
): string {
  if (busy) return t("imports.upload.starting");
  if (pasting) return pasteHint(paste, questions, max, t, locale);
  return canStart ? t("imports.upload.readyHint") : t("imports.upload.chooseHint");
}

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
 * queues processing. For a new import, `fileMode` false shows the pasted-text
 * box in place of the files; the files, the text and a typed title survive a
 * switch, and leaving with text that has not started asks first.
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
  const locale = useLocale();
  const released = useRef(false);
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
    onStarted: (started) => {
      released.current = true;
      onStarted(started);
    },
  });
  const [typedTitle, setTypedTitle] = useState<string | null>(null);
  const pasteMax = limits.data?.pasteMaxCharacters ?? PASTE_MAX;
  const paste = usePastedText(pasteMax);
  const pasting = existing === null && !fileMode;
  const examName = intake.slots.exam.file?.name ?? intake.slots.exam.received?.filename;
  const suggested = examName === undefined ? "" : titleFromFilename(examName);
  const title = typedTitle ?? (pasting ? paste.firstLine : suggested);
  const askTitle = existing === null;
  const titleLocked = intake.importId !== null;
  const accept = (limits.data?.formats ?? ["docx"])
    .map((format) => `.${format}`)
    .join(",");
  const filesReady = intake.ready && (!askTitle || title.trim() !== "");
  const pasteReady = !intake.busy && paste.state === "ready";
  const canStart = enabled && (pasting ? pasteReady : filesReady);
  const fallbackTitle = t("imports.paste.fallbackTitle", {
    date: dayMonth(new Date(), locale),
  });
  const hint = footerHint(
    {
      busy: intake.busy,
      pasting,
      canStart,
      paste: paste.state,
      questions: paste.scan.questions,
      max: pasteMax,
    },
    t,
    locale,
  );

  function submit() {
    if (!canStart) return;
    if (!pasting) {
      void intake.start(title.trim());
      return;
    }
    const exact = paste.sendable();
    if (exact !== null) void intake.startText(title.trim() || fallbackTitle, exact);
  }

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
        submit();
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
        <>
          <div hidden={!pasting}>
            <PasteField
              text={paste.text}
              length={paste.length}
              max={pasteMax}
              scan={paste.scan}
              busy={intake.busy}
              onTextChange={paste.setText}
            />
          </div>
          <TitleField
            value={title}
            disabled={intake.busy}
            locked={titleLocked}
            pasting={pasting}
            fallback={fallbackTitle}
            onChange={setTypedTitle}
          />
        </>
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
        hint={hint}
        canStart={canStart}
        busy={intake.busy}
        replacing={replacing}
      />
      {askTitle ? <LeaveGuard dirty={paste.text !== ""} released={released} /> : null}
    </form>
  );
}

function TitleField({
  value,
  disabled,
  locked,
  pasting,
  fallback,
  onChange,
}: Readonly<{
  value: string;
  disabled: boolean;
  locked: boolean;
  pasting: boolean;
  fallback: string;
  onChange: (next: string) => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  let hint = pasting ? t("imports.paste.titleHint") : t("imports.upload.titleHint");
  if (locked) hint = t("imports.upload.titleLocked");
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{t("imports.upload.titleLabel")}</Label>
      <Input
        id={id}
        value={value}
        maxLength={TITLE_MAX}
        disabled={locked || disabled}
        placeholder={pasting ? fallback : t("imports.upload.titlePlaceholder")}
        onChange={(event) => onChange(event.target.value)}
      />
      <p className="text-muted-foreground text-xs">{hint}</p>
    </div>
  );
}

function LeaveGuard({
  dirty,
  released,
}: Readonly<{ dirty: boolean; released: RefObject<boolean> }>) {
  const { t } = useTranslation();
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirty && !released.current && currentLocation.pathname !== nextLocation.pathname,
  );
  useBeforeUnload(
    useCallback(
      (event: BeforeUnloadEvent) => {
        if (dirty && !released.current) event.preventDefault();
      },
      [dirty, released],
    ),
  );
  return (
    <ConfirmDialog
      open={blocker.state === "blocked"}
      onOpenChange={(open) => {
        if (!open && blocker.state === "blocked") blocker.reset();
      }}
      title={t("imports.paste.leaveTitle")}
      description={t("imports.paste.leaveBody")}
      confirmLabel={t("imports.paste.leave")}
      cancelLabel={t("imports.paste.stay")}
      destructive
      onConfirm={() => {
        if (blocker.state === "blocked") blocker.proceed();
      }}
    />
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
  hint,
  canStart,
  busy,
  replacing,
}: Readonly<{
  deck: boolean;
  hint: string;
  canStart: boolean;
  busy: boolean;
  replacing: boolean;
}>) {
  const { t } = useTranslation();
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
