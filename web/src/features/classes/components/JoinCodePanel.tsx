import { useMemo, useRef, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { Copy, Download, Link2, Pause, RotateCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/components/ui/sonner";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import {
  getJoinCode,
  revokeJoinCode,
  rotateJoinCode,
  updateClass,
  type Class,
} from "@/features/classes/api";
import { saveError } from "@/features/classes/classFields";
import { JoinQr } from "@/features/classes/components/JoinQr";
import { JoinQrDialog } from "@/features/classes/components/JoinQrDialog";
import { invalidateClass } from "@/features/classes/invalidate";
import {
  joinLinkLabel,
  joinLinkOf,
  useCopyJoinCode,
  useCopyJoinLink,
} from "@/features/classes/joinLink";
import { downloadJoinQr } from "@/features/classes/joinQrImage";
import { dayMonthYear, formatTime, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

const EXPIRY_DAYS = ["30", "7", "90"] as const;
const DEFAULT_MAX_USES = "40";
const MAX_USES = 1000;
const DAY_MS = 86_400_000;
const QR_BG = "var(--qr-bg)";
const QR_FG = "var(--qr-fg)";

type CodeOptions = { expires: string; maxUses: string };

const FIRST = {
  button: "classDetail.issue",
  title: "classDetail.issueConfirmTitle",
  body: "classDetail.issueConfirmBody",
  submit: "classDetail.issue",
} as const;
const NEXT = {
  button: "classDetail.rotate",
  title: "classDetail.rotateConfirmTitle",
  body: "classDetail.rotateConfirmBody",
  submit: "classDetail.rotateConfirm",
} as const;

const SHARE =
  "bg-card hover:bg-muted inline-flex h-8 min-w-0 rounded-md flex-[1_1_auto] items-center justify-center gap-1.5 border px-2.5 text-meta font-medium whitespace-nowrap [&_svg]:size-3.5 [&_svg]:flex-none";

/**
 * JoinCodePanel is the class's join code card, as the v2 deck draws it
 * (DG-01): the "Students can join" switch; the QR as a button that opens it
 * full size for the projector, faded while joining is paused; the code
 * always in full (read with `getJoinCode`, kept no longer than the screen)
 * with the join link under it and, while paused, a "Joining paused" chip;
 * Copy code, Copy link and Download QR; when it expires and how many uses it
 * has had; New code with its expiry and use limit (30, 7 or 90 days; 1 to
 * 1,000 uses, 40 by default), which then shows the new code in the New join
 * code dialog; and Stop after a confirm. A class with no code says so and
 * offers Create code, without the line that speaks of an old code. An
 * archived class cannot be joined,
 * so it offers neither New code nor Create code and says to restore it
 * first; Stop stays. A page that has not yet seen the class archived
 * elsewhere still offers New code; the server's 409 CLASS_ARCHIVED then
 * shows its own message in the dialog and the class is read again. A code that cannot be read shows its last four
 * characters until a new one is made. Without `canWrite` the card only
 * reads, the code masked.
 */
export function JoinCodePanel({
  klass,
  canWrite,
}: Readonly<{ klass: Class; canWrite: boolean }>) {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const info = klass.joinCode ?? null;
  const code = useQuery({
    queryKey: ["teacher-class", klass.id, "code"],
    queryFn: ({ signal }) => getJoinCode(klass.id, signal),
    enabled: canWrite && info !== null,
    gcTime: 0,
  });
  const readable = info === null ? null : (code.data?.code ?? null);
  const words = info === null ? FIRST : NEXT;
  const [now] = useState(() => Date.now());
  const archived = klass.archivedAt !== null;
  const rotateButton = useRef<HTMLButtonElement>(null);
  const switchControl = useRef<HTMLButtonElement>(null);
  const [rotating, setRotating] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [showingQr, setShowingQr] = useState(false);
  const [issued, setIssued] = useState<string | null>(null);

  const refresh = () => invalidateClass(queryClient, klass.id);
  const selfJoin = useMutation({
    mutationFn: (enabled: boolean) =>
      updateClass(klass.id, { selfJoinEnabled: enabled }),
    onSuccess: async (_, enabled) => {
      toast(t(enabled ? "classDetail.selfJoinOn" : "classDetail.selfJoinOff"));
      await refresh();
    },
    onError: () => toast.error(t("classDetail.selfJoinFailed")),
  });
  const rotate = useMutation({
    mutationFn: (options: CodeOptions) =>
      rotateJoinCode(klass.id, {
        expiresInDays: Number(options.expires),
        maxUses: Number(options.maxUses),
      }),
    onSuccess: async (made) => {
      setRotating(false);
      setIssued(made.code);
      await refresh();
    },
    onError: (cause) => {
      if (cause instanceof ApiError && cause.code === "CLASS_ARCHIVED") void refresh();
    },
  });
  const revoke = useMutation({
    mutationFn: () => revokeJoinCode(klass.id),
    onSuccess: async () => {
      setStopping(false);
      toast(t("classDetail.stopped"));
      await refresh();
    },
  });

  const fields = useMemo<readonly FormField<CodeOptions>[]>(
    () => [
      {
        kind: "select",
        name: "expires",
        label: t("classDetail.expiryLabel"),
        options: EXPIRY_DAYS.map((days) => ({
          value: days,
          label: t("classDetail.expiryDays", { days }),
        })),
      },
      {
        kind: "number",
        name: "maxUses",
        label: t("classDetail.maxUsesLabel"),
        hint: t("classDetail.maxUsesHint"),
        inputMode: "numeric",
        required: true,
      },
    ],
    [t],
  );

  return (
    <section
      aria-labelledby="join-code-heading"
      className="bg-card shadow-card rounded-xl border"
    >
      <div className="flex flex-wrap items-center justify-between gap-3 px-4.5 pt-4 pb-3">
        <h2
          id="join-code-heading"
          className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
        >
          {t("classDetail.joinCode")}
        </h2>
        <label className="text-meta flex items-center gap-2">
          {t("classDetail.selfJoinSwitch")}
          <Switch
            ref={switchControl}
            size="sm"
            checked={klass.selfJoinEnabled}
            disabled={!canWrite}
            aria-disabled={selfJoin.isPending || undefined}
            className="aria-disabled:opacity-50"
            onCheckedChange={(next) => {
              if (!selfJoin.isPending) selfJoin.mutate(next);
            }}
          />
        </label>
      </div>

      <div className="flex flex-col gap-3.5 px-4.5 pb-4">
        {info === null ? (
          <p className="text-muted-fg text-sm">{t("classDetail.noActiveCode")}</p>
        ) : (
          <CodeSummary
            info={info}
            readable={readable}
            unreadable={canWrite && code.isSuccess && readable === null}
            paused={!klass.selfJoinEnabled}
            now={now}
            onShowQr={() => setShowingQr(true)}
          />
        )}
        {canWrite ? (
          <CodeActions
            archived={archived}
            hasCode={info !== null}
            rotateLabel={t(words.button)}
            rotateRef={rotateButton}
            onRotate={() => {
              rotate.reset();
              setRotating(true);
            }}
            onStop={() => {
              revoke.reset();
              setStopping(true);
            }}
          />
        ) : null}
        {info === null || archived ? null : (
          <p className="text-muted-fg text-xs leading-normal">
            {t("classDetail.newCodeNote")}
          </p>
        )}
      </div>

      <FormDialog
        open={rotating}
        onOpenChange={setRotating}
        title={t(words.title)}
        description={t(words.body)}
        icon={RotateCw}
        initial={{ expires: "30", maxUses: DEFAULT_MAX_USES }}
        fields={fields}
        validate={(values) =>
          maxUsesValid(values.maxUses)
            ? null
            : { maxUses: t("classDetail.maxUsesRange") }
        }
        submitLabel={t(words.submit)}
        pending={rotate.isPending}
        error={
          rotate.isError ? saveError(rotate.error, t("classDetail.rotateFailed")) : null
        }
        onSubmit={(values) => {
          if (!rotate.isPending) rotate.mutate(values);
        }}
      />
      <ConfirmDialog
        open={stopping}
        onOpenChange={setStopping}
        title={t("classDetail.revokeConfirmTitle")}
        description={t("classDetail.revokeConfirmBody")}
        confirmLabel={t("classDetail.revokeConfirm")}
        destructive
        pending={revoke.isPending}
        error={
          revoke.isError ? saveError(revoke.error, t("classDetail.revokeFailed")) : null
        }
        returnFocus={archived ? switchControl : rotateButton}
        onConfirm={() => {
          if (!revoke.isPending) revoke.mutate();
        }}
      />
      <JoinQrDialog
        code={showingQr ? readable : null}
        title={t("classDetail.qrDialogTitle", { name: klass.name })}
        description={t(
          klass.selfJoinEnabled
            ? "classDetail.qrDialogHint"
            : "classDetail.qrDialogPaused",
        )}
        onClose={() => setShowingQr(false)}
      />
      <JoinQrDialog
        code={issued}
        share
        title={t("classDetail.issuedTitle")}
        description={t("classDetail.issuedHint")}
        returnFocus={rotateButton}
        onClose={() => setIssued(null)}
      />
    </section>
  );
}

function CodeActions({
  archived,
  hasCode,
  rotateLabel,
  rotateRef,
  onRotate,
  onStop,
}: Readonly<{
  archived: boolean;
  hasCode: boolean;
  rotateLabel: string;
  rotateRef: RefObject<HTMLButtonElement | null>;
  onRotate: () => void;
  onStop: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <>
      {archived ? (
        <p className="text-muted-fg text-xs leading-normal">
          {t("classDetail.archivedNoCode")}
        </p>
      ) : null}
      {archived && !hasCode ? null : (
        <div className="flex gap-2">
          {archived ? null : (
            <Button
              ref={rotateRef}
              variant="outline"
              className="h-8.5 flex-1 in-data-[scale=deck]:h-8.5 in-data-[scale=deck]:text-sm"
              onClick={onRotate}
            >
              <RotateCw aria-hidden="true" className="size-3.5" />
              {rotateLabel}
            </Button>
          )}
          {hasCode ? (
            <Button
              variant="ghost"
              className="text-muted-fg h-8.5 font-normal in-data-[scale=deck]:h-8.5 in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm"
              onClick={onStop}
            >
              {t("classDetail.stop")}
            </Button>
          ) : null}
        </div>
      )}
    </>
  );
}

function CodeSummary({
  info,
  readable,
  unreadable,
  paused,
  now,
  onShowQr,
}: Readonly<{
  info: NonNullable<Class["joinCode"]>;
  readable: string | null;
  unreadable: boolean;
  paused: boolean;
  now: number;
  onShowQr: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const copyLink = useCopyJoinLink();
  const copyCode = useCopyJoinCode();
  const qrFrame = useRef<HTMLDivElement>(null);
  return (
    <>
      <div className="flex flex-wrap items-center gap-3.5">
        {readable === null ? null : (
          <QrButton code={readable} paused={paused} onClick={onShowQr} />
        )}
        <div className="flex min-w-0 flex-[1_1_140px] flex-col items-start gap-1">
          <p className="text-stat font-mono leading-[1.3] font-semibold tracking-[.08em] whitespace-nowrap">
            {readable ?? t("classDetail.maskedCode", { hint: info.hint })}
          </p>
          {readable === null ? null : (
            <p className="text-muted-fg text-meta max-w-full truncate leading-normal">
              {joinLinkLabel(readable)}
            </p>
          )}
          {unreadable ? (
            <p className="text-muted-fg text-xs leading-[1.45]">
              {t("classDetail.unreadableNote")}
            </p>
          ) : null}
          {paused ? (
            <Badge
              variant="warning"
              className="mt-0.5 max-w-full border-0 in-data-[scale=deck]:px-2 [&>svg]:size-3"
            >
              <Pause aria-hidden="true" />
              <span className="truncate">{t("classDetail.joiningPaused")}</span>
            </Badge>
          ) : null}
        </div>
      </div>
      {readable === null ? null : (
        <div className="flex flex-wrap gap-1.5">
          <button type="button" className={SHARE} onClick={() => copyCode(readable)}>
            <Copy aria-hidden="true" />
            <span className="truncate">{t("classDetail.copyCode")}</span>
          </button>
          <button type="button" className={SHARE} onClick={() => copyLink(readable)}>
            <Link2 aria-hidden="true" />
            <span className="truncate">{t("classDetail.copyLink")}</span>
          </button>
          <button
            type="button"
            className={SHARE}
            onClick={() => downloadJoinQr(qrFrame.current, readable)}
          >
            <Download aria-hidden="true" />
            <span className="truncate">{t("classDetail.downloadQr")}</span>
          </button>
          <div hidden>
            <JoinQr ref={qrFrame} code={readable} />
          </div>
        </div>
      )}
      <dl className="text-ui m-0 flex flex-col gap-2 border-t pt-3 leading-normal">
        <div className="flex justify-between gap-3">
          <dt className="text-muted-fg">{t("classDetail.expiresAt")}</dt>
          <dd className="m-0 text-right">
            {t("classDetail.expiresOn", {
              date: dayMonthYear(info.expiresAt, locale),
              time: formatTime(info.expiresAt, locale),
            })}{" "}
            <ExpiryNote expiresAt={info.expiresAt} now={now} />
          </dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-muted-fg">{t("classDetail.uses")}</dt>
          <dd className="m-0 tabular-nums">
            {t("classDetail.usesValue", {
              used: info.usesCount,
              max: info.maxUses ?? t("classDetail.unlimited"),
            })}
          </dd>
        </div>
      </dl>
    </>
  );
}

function QrButton({
  code,
  paused,
  onClick,
}: Readonly<{ code: string; paused: boolean; onClick: () => void }>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-label={t("classDetail.showQr")}
      title={t("classDetail.showQrFull")}
      className={cn(
        "bg-qr-bg text-qr-fg grid size-28 flex-none cursor-zoom-in place-items-center rounded-[10px] border p-0 transition-opacity motion-reduce:transition-none",
        paused && "opacity-40",
      )}
      onClick={onClick}
    >
      <QRCodeSVG
        value={joinLinkOf(code)}
        size={88}
        level="M"
        bgColor={QR_BG}
        fgColor={QR_FG}
        aria-hidden="true"
      />
    </button>
  );
}

function ExpiryNote({ expiresAt, now }: Readonly<{ expiresAt: string; now: number }>) {
  const { t } = useTranslation();
  const left = Date.parse(expiresAt) - now;
  if (left <= 0)
    return <span className="text-danger-ink">{t("classDetail.expiredNote")}</span>;
  const days = Math.floor(left / DAY_MS);
  return (
    <span className="text-muted-fg">
      {days === 0
        ? t("classDetail.expiresToday")
        : t("classDetail.expiresIn", { count: days })}
    </span>
  );
}

function maxUsesValid(value: string): boolean {
  const n = Number(value.trim());
  return value.trim() !== "" && Number.isInteger(n) && n >= 1 && n <= MAX_USES;
}
