import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { Copy, Download, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/components/ui/sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import {
  revokeJoinCode,
  rotateJoinCode,
  updateClass,
  type Class,
  type JoinCodeOptions,
} from "@/features/classes/api";
import { invalidateClass } from "@/features/classes/invalidate";
import { formatDateTime } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { ApiError } from "@/lib/api/errors";

// 30 days is preselected because it is the contract's own default; 1000 is its ceiling.
const EXPIRY_CHOICES = [7, 30, 90] as const;
const DEFAULT_EXPIRY_DAYS = 30;
const MAX_USES_CEILING = 1000;

/** §6.4's join-code panel. */
export function JoinCodePanel({ klass }: Readonly<{ klass: Class }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const [freshCode, setFreshCode] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<"rotate" | "revoke" | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expiresInDays, setExpiresInDays] = useState(DEFAULT_EXPIRY_DAYS);
  const [maxUses, setMaxUses] = useState("");

  const [openedAt] = useState(() => Date.now());
  const code = klass.joinCode;
  const expired = code ? Date.parse(code.expiresAt) <= openedAt : false;
  const exhausted = code
    ? code.maxUses !== null && code.usesCount >= code.maxUses
    : false;
  const spent = expired || exhausted;

  const wantedMaxUses = maxUses.trim() === "" ? null : Number(maxUses.trim());
  const maxUsesInvalid =
    wantedMaxUses !== null &&
    (!Number.isInteger(wantedMaxUses) ||
      wantedMaxUses < 1 ||
      wantedMaxUses > MAX_USES_CEILING);

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["admin-class", klass.id] });

  const rotate = useMutation({
    mutationFn: (options: JoinCodeOptions) => rotateJoinCode(klass.id, options),
    onSuccess: async (result) => {
      setFreshCode(result.code);
      setConfirming(null);
      setError(null);
      setCopied(false);
      await invalidate();
    },
    onError: (cause) => {
      setConfirming(null);
      setError(
        cause instanceof ApiError ? cause.message : t("classDetail.rotateFailed"),
      );
    },
  });

  // G-06's "Cho tham gia": pausing enrolment without reissuing the code (§6.4).
  const selfJoin = useMutation({
    mutationFn: (enabled: boolean) =>
      updateClass(klass.id, { selfJoinEnabled: enabled }),
    onSuccess: async (_, enabled) => {
      await invalidateClass(queryClient, klass.id);
      toast(t(enabled ? "classDetail.selfJoinOn" : "classDetail.selfJoinOff"));
    },
    onError: () => toast(t("classDetail.selfJoinFailed")),
  });

  const revoke = useMutation({
    mutationFn: () => revokeJoinCode(klass.id),
    onSuccess: async () => {
      setFreshCode(null);
      setConfirming(null);
      setCopied(false);
      setError(null);
      await invalidate();
    },
    onError: (cause) => {
      setConfirming(null);
      setError(
        cause instanceof ApiError ? cause.message : t("classDetail.revokeFailed"),
      );
    },
  });

  const joinUrl = freshCode
    ? `${window.location.origin}/join/${freshCode.replace("-", "")}`
    : null;
  const locale = useLocale();

  function openRotate() {
    setExpiresInDays(DEFAULT_EXPIRY_DAYS);
    setMaxUses("");
    setConfirming("rotate");
  }

  function submitRotate() {
    if (maxUsesInvalid) return;
    const options: JoinCodeOptions = { expiresInDays };
    if (wantedMaxUses !== null) options.maxUses = wantedMaxUses;
    rotate.mutate(options);
  }

  function copyCode(value: string) {
    const clipboard = navigator.clipboard as Clipboard | undefined;
    if (!clipboard) {
      setError(t("classDetail.copyFailed"));
      return;
    }
    void clipboard.writeText(value).then(
      () => toast(t("classDetail.codeCopied")),
      () => setError(t("classDetail.copyFailed")),
    );
  }

  function copyJoinUrl(url: string) {
    const clipboard = navigator.clipboard as Clipboard | undefined;
    if (!clipboard) {
      setCopied(false);
      setError(t("classDetail.copyFailed"));
      return;
    }
    void clipboard.writeText(url).then(
      () => {
        setError(null);
        setCopied(true);
      },
      () => {
        setCopied(false);
        setError(t("classDetail.copyFailed"));
      },
    );
  }

  return (
    <Card asChild className="gap-0 py-0">
      <section aria-labelledby="join-code-heading">
        <div className="flex items-center justify-between gap-3 px-5 pt-4 pb-3">
          <h2
            id="join-code-heading"
            className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
          >
            {t("classDetail.joinCode")}
          </h2>
          <label className="flex items-center gap-2 text-xs">
            {t("classDetail.selfJoinSwitch")}
            <Switch
              checked={klass.selfJoinEnabled}
              disabled={selfJoin.isPending}
              aria-label={t("classDetail.selfJoinSwitch")}
              onCheckedChange={(next) => selfJoin.mutate(next)}
            />
          </label>
        </div>

        <div className="space-y-3 px-5 pb-4">
          {klass.selfJoinEnabled ? null : (
            <p className="text-muted-foreground text-xs leading-relaxed">
              {t("classDetail.selfJoinOffHint")}
            </p>
          )}
          {code ? (
            <CodeSummary
              code={code}
              expired={expired}
              exhausted={exhausted}
              locale={locale}
            />
          ) : (
            <p className="text-muted-foreground text-sm">
              {t("classDetail.noActiveCode")}
            </p>
          )}

          {error ? (
            <p role="alert" className="text-destructive text-sm">
              {error}
            </p>
          ) : null}

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="flex-1"
              onClick={openRotate}
              disabled={rotate.isPending}
            >
              <RotateCw aria-hidden="true" />
              {code && !spent ? t("classDetail.rotate") : t("classDetail.issue")}
            </Button>
            {code ? (
              <Button
                variant="ghost"
                size="sm"
                className="text-muted-foreground"
                aria-label={t("classDetail.disableSelfJoin")}
                onClick={() => setConfirming("revoke")}
                disabled={revoke.isPending}
              >
                {t("classDetail.stop")}
              </Button>
            ) : null}
          </div>

          <p className="text-muted-foreground text-xs leading-relaxed">
            {t(explainerKey(Boolean(code), expired, exhausted))}
          </p>
        </div>

        <ConfirmDialog
          open={confirming === "rotate"}
          onOpenChange={(open) => !open && setConfirming(null)}
          title={t(
            code ? "classDetail.rotateConfirmTitle" : "classDetail.issueConfirmTitle",
          )}
          description={t(
            code ? "classDetail.rotateConfirmBody" : "classDetail.issueConfirmBody",
          )}
          confirmLabel={t("classDetail.rotateConfirm")}
          disabled={maxUsesInvalid}
          pending={rotate.isPending}
          error={maxUsesInvalid ? t("classDetail.maxUsesRange") : null}
          onConfirm={submitRotate}
        >
          <CodeOptionsFields
            expiresInDays={expiresInDays}
            maxUses={maxUses}
            maxUsesInvalid={maxUsesInvalid}
            onExpiryChange={setExpiresInDays}
            onMaxUsesChange={setMaxUses}
          />
        </ConfirmDialog>

        <ConfirmDialog
          open={confirming === "revoke"}
          onOpenChange={(open) => !open && setConfirming(null)}
          title={t("classDetail.revokeConfirmTitle")}
          description={t("classDetail.revokeConfirmBody")}
          confirmLabel={t("classDetail.revokeConfirm")}
          destructive
          pending={revoke.isPending}
          onConfirm={() => revoke.mutate()}
        />

        <FreshCodeDialog
          code={freshCode}
          joinUrl={joinUrl}
          copied={copied}
          onCopy={copyJoinUrl}
          onCopyCode={copyCode}
          onClose={() => setFreshCode(null)}
        />
      </section>
    </Card>
  );
}

function CodeOptionsFields({
  expiresInDays,
  maxUses,
  maxUsesInvalid,
  onExpiryChange,
  onMaxUsesChange,
}: Readonly<{
  expiresInDays: number;
  maxUses: string;
  maxUsesInvalid: boolean;
  onExpiryChange: (days: number) => void;
  onMaxUsesChange: (value: string) => void;
}>) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label htmlFor="join-code-expiry">{t("classDetail.expiryLabel")}</Label>
        <Select
          value={String(expiresInDays)}
          onValueChange={(next) => onExpiryChange(Number(next))}
        >
          <SelectTrigger id="join-code-expiry" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {EXPIRY_CHOICES.map((days) => (
              <SelectItem key={days} value={String(days)}>
                {t("classDetail.expiryDays", { days })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="join-code-max-uses">{t("classDetail.maxUsesLabel")}</Label>
        <Input
          id="join-code-max-uses"
          type="number"
          inputMode="numeric"
          min={1}
          max={MAX_USES_CEILING}
          value={maxUses}
          aria-invalid={maxUsesInvalid}
          aria-describedby="join-code-max-uses-hint"
          placeholder={t("classDetail.maxUsesPlaceholder")}
          onChange={(event) => onMaxUsesChange(event.target.value)}
        />
        <p
          id="join-code-max-uses-hint"
          className="text-muted-foreground text-xs leading-relaxed"
        >
          {t("classDetail.maxUsesHint")}
        </p>
      </div>
    </div>
  );
}

function CodeSummary({
  code,
  expired,
  exhausted,
  locale,
}: Readonly<{
  code: NonNullable<Class["joinCode"]>;
  expired: boolean;
  exhausted: boolean;
  locale: Locale;
}>) {
  const { t } = useTranslation();

  const days = daysUntil(code.expiresAt);

  return (
    <>
      <div className="rounded-md border p-3 text-center">
        <p className="font-mono text-xl tracking-wide">
          {t("classDetail.maskedCode", { hint: code.hint })}
        </p>
        <p className="text-muted-foreground mt-1.5 text-xs leading-relaxed">
          {t("classDetail.codeMaskedNote")}
        </p>
      </div>

      <dl className="space-y-2 text-sm">
        <div className="flex items-center justify-between gap-3">
          <dt className="text-muted-foreground">{t("classDetail.expiresAt")}</dt>
          <dd className="text-right">
            {formatDateTime(code.expiresAt, locale)}{" "}
            {expired ? (
              <span className="text-destructive text-xs font-medium">
                {t("classDetail.expiredBadge")}
              </span>
            ) : (
              <span className="text-muted-foreground">
                {days === 0
                  ? t("classDetail.expiresToday")
                  : t("classDetail.expiresIn", { count: days })}
              </span>
            )}
          </dd>
        </div>
        <div className="flex items-center justify-between gap-3">
          <dt className="text-muted-foreground">{t("classDetail.uses")}</dt>
          <dd className="flex items-center gap-2">
            <span className="tabular-nums">
              {code.usesCount}
              {" / "}
              {code.maxUses === null ? t("classDetail.unlimited") : code.maxUses}
            </span>
            {exhausted ? (
              <span className="text-destructive text-xs font-medium">
                {t("classDetail.exhaustedBadge")}
              </span>
            ) : null}
          </dd>
        </div>
      </dl>
    </>
  );
}

/**
 * G-06's "Tải QR": the projector and the Zalo group want an image, so the
 * on-screen SVG is rasterised through a canvas; where there is no canvas the
 * SVG itself is what gets saved.
 */
function downloadQr(host: HTMLDivElement | null, filename: string) {
  const svg = host?.querySelector("svg");
  if (!svg) return;
  const markup = new XMLSerializer().serializeToString(svg);
  const svgBlob = new Blob([markup], { type: "image/svg+xml" });
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 512;
  const context = canvas.getContext("2d");
  if (!context) {
    save(svgBlob, filename.replace(/\.png$/, ".svg"));
    return;
  }
  const image = new Image();
  const url = URL.createObjectURL(svgBlob);
  image.onload = () => {
    context.fillStyle = "#ffffff";
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, 32, 32, 448, 448);
    URL.revokeObjectURL(url);
    canvas.toBlob((png) => png && save(png, filename), "image/png");
  };
  image.src = url;
}

function save(blob: Blob, filename: string) {
  const anchor = document.createElement("a");
  anchor.href = URL.createObjectURL(blob);
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(anchor.href);
}

// The deck's "còn 13 ngày": what makes §6.5's 30-day default read as safety, not friction.
function daysUntil(expiresAt: string): number {
  const ms = new Date(expiresAt).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / 86_400_000));
}

/** The one moment the plaintext code exists (§13.3): dismissing this is the last look. */
function FreshCodeDialog({
  code,
  joinUrl,
  copied,
  onCopy,
  onCopyCode,
  onClose,
}: Readonly<{
  code: string | null;
  joinUrl: string | null;
  copied: boolean;
  onCopy: (joinUrl: string) => void;
  onCopyCode: (code: string) => void;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const qr = useRef<HTMLDivElement>(null);

  return (
    <Dialog open={code !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        showCloseButton={false}
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>{t("classDetail.freshTitle")}</DialogTitle>
          <DialogDescription>{t("classDetail.rotateConfirmBody")}</DialogDescription>
        </DialogHeader>

        <div className="rounded-md border p-4 text-center">
          <p className="font-mono text-2xl tracking-wide">{code}</p>
          <p className="text-muted-foreground mt-2 text-xs">
            {t("classDetail.shownOnce")}
          </p>
        </div>

        {joinUrl && code ? (
          <div className="flex items-center gap-3">
            <div ref={qr}>
              <QRCodeSVG
                value={joinUrl}
                size={80}
                level="M"
                aria-label={t("classDetail.qrAlt")}
              />
            </div>
            <div className="min-w-0 space-y-1.5">
              <p className="text-muted-foreground font-mono text-xs break-words">
                {joinUrl}
              </p>
              <div className="flex flex-wrap gap-1.5">
                <Button variant="outline" size="xs" onClick={() => onCopy(joinUrl)}>
                  <Copy aria-hidden="true" />
                  {copied ? t("classDetail.copied") : t("classDetail.copyLink")}
                </Button>
                <Button variant="outline" size="xs" onClick={() => onCopyCode(code)}>
                  <Copy aria-hidden="true" />
                  {t("classDetail.copyCode")}
                </Button>
                <Button
                  variant="outline"
                  size="xs"
                  onClick={() => downloadQr(qr.current, `quizzivy-${code}.png`)}
                >
                  <Download aria-hidden="true" />
                  {t("classDetail.downloadQr")}
                </Button>
              </div>
            </div>
          </div>
        ) : null}

        <DialogFooter>
          <Button className="w-full" onClick={onClose}>
            {t("classDetail.done")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function explainerKey(hasCode: boolean, expired: boolean, exhausted: boolean): string {
  if (hasCode && expired) return "classDetail.expiredExplainer";
  if (hasCode && exhausted) return "classDetail.exhaustedExplainer";
  return "classDetail.codeShareHint";
}
