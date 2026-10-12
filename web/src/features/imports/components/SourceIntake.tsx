import {
  useCallback,
  useId,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { Link, useBeforeUnload, useBlocker } from "react-router";
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
import { listFormats } from "../formats";
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

  return (
    <form
      className={cn("flex flex-col", deck ? "gap-4.5" : "gap-6")}
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <div hidden={!fileMode}>
        <SourceFiles
          deck={deck}
          pending={limits.isPending}
          failed={limits.isError}
          maxBytes={limits.data?.maxBytes}
          formats={limits.data?.formats}
          onRetry={() => void limits.refetch()}
        >
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
        </SourceFiles>
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
            deck={deck}
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
  deck,
  value,
  disabled,
  locked,
  pasting,
  fallback,
  onChange,
}: Readonly<{
  deck: boolean;
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
    <div
      className={cn(
        "flex flex-col gap-1.5",
        deck &&
          "bg-card text-card-foreground shadow-card rounded-xl border px-4.5 py-4",
      )}
    >
      <Label htmlFor={id}>{t("imports.upload.titleLabel")}</Label>
      <Input
        id={id}
        value={value}
        maxLength={TITLE_MAX}
        className="max-w-(--field-max)"
        disabled={locked || disabled}
        placeholder={pasting ? fallback : t("imports.upload.titlePlaceholder")}
        onChange={(event) => onChange(event.target.value)}
      />
      <p className="text-muted-fg text-meta leading-normal">{hint}</p>
      {deck ? (
        <p className="text-muted-fg text-meta mt-1.5 border-t pt-2.5 leading-[1.55] text-pretty">
          {t("imports.upload.recognitionNote")}
        </p>
      ) : null}
    </div>
  );
}

function SourceFiles({
  deck,
  pending,
  failed,
  maxBytes,
  formats,
  onRetry,
  children,
}: Readonly<{
  deck: boolean;
  pending: boolean;
  failed: boolean;
  maxBytes: number | undefined;
  formats: readonly string[] | undefined;
  onRetry: () => void;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  const headingId = useId();
  const limits = (
    <LimitsLine
      pending={pending}
      failed={failed}
      maxBytes={maxBytes}
      formats={formats}
      onRetry={onRetry}
    />
  );
  if (!deck)
    return (
      <>
        {limits}
        <div className="flex flex-col gap-5">{children}</div>
      </>
    );
  return (
    <section
      aria-labelledby={headingId}
      className="bg-card text-card-foreground shadow-card rounded-xl border"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-3 px-4.5 pt-4 pb-1">
        <h2 id={headingId} className="text-md font-semibold">
          {t("imports.newHeading")}
        </h2>
        {maxBytes === undefined || formats === undefined ? null : (
          <span className="text-muted-fg text-meta">
            {t("imports.newFormats", {
              formats: listFormats(formats, "name", "or", t),
              size: formatBytes(maxBytes),
            })}
          </span>
        )}
      </div>
      <div className="flex flex-col gap-3.5 px-4.5 pt-2.5 pb-4.5">
        {limits}
        {children}
      </div>
    </section>
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
  const headingId = useId();
  if (replacing) return null;
  if (!deck)
    return (
      <p className="text-muted-foreground text-xs leading-relaxed">
        {t("imports.upload.recognitionNote")}
      </p>
    );
  return (
    <section
      aria-labelledby={headingId}
      className="bg-card text-card-foreground shadow-card rounded-xl border"
    >
      <h2 id={headingId} className="text-md px-4.5 pt-4 pb-1 font-semibold">
        {t("imports.upload.recognitionHeading")}
      </h2>
      <div className="@container/modes px-4.5 pt-2.5 pb-4.5">
        <div className="grid gap-2.5 @[490px]/modes:grid-cols-2">
          <p className="border-primary flex items-start gap-3 rounded-[10px] border-[1.5px] px-3.5 py-3">
            <span
              aria-hidden="true"
              className="border-primary mt-px grid size-4.5 shrink-0 place-items-center rounded-full border-2"
            >
              <span className="bg-primary size-2 rounded-full" />
            </span>
            <span>
              <span className="text-ui block leading-[normal] font-medium">
                {t("imports.upload.automatic")}
              </span>
              <span className="text-muted-fg text-meta block leading-[1.45]">
                {t("imports.upload.automaticHint")}
              </span>
            </span>
          </p>
        </div>
      </div>
    </section>
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
        "flex flex-wrap items-center",
        deck ? "bg-background sticky bottom-0 gap-2.5 border-t py-3" : "gap-3",
      )}
    >
      {deck ? (
        <>
          <span className="text-muted-fg text-meta min-w-0 flex-[1_1_220px] leading-normal">
            {hint}
          </span>
          <Button
            asChild
            type="button"
            variant="outline"
            size="md"
            className="leading-[normal]"
          >
            <Link to="/teacher/imports">{t("common.cancel")}</Link>
          </Button>
        </>
      ) : null}
      <Button
        type="submit"
        size={deck ? "md" : "default"}
        className={deck ? "px-4 leading-[normal] font-semibold" : undefined}
        disabled={!canStart}
      >
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
    <p className="bg-muted rounded-md px-3 py-2.5 text-sm leading-[1.55]">
      {t("imports.upload.limits", {
        formats: listFormats(formats, "extension", "and", t),
        size: formatBytes(maxBytes),
      })}
    </p>
  );
}
