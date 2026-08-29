import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { Copy, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { revokeJoinCode, rotateJoinCode, type Class } from "@/features/classes/api";
import { formatDateTime } from "@/lib/i18n/datetime";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { ApiError } from "@/lib/api/errors";

/**
 * §6.4's join-code panel.
 *
 * The plaintext code exists in exactly one place: the response to a rotation,
 * held in component state for as long as the teacher is looking at it. It is
 * never written to the query cache (which survives navigation) or to storage,
 * and there is no endpoint that could return it again -- only a SHA-256 hash is
 * stored (§13.3). So the panel has two shapes, and which one you see is
 * determined by whether you just pressed the button.
 */
/** i18next hands back a plain string; the formatter wants one of ours. */
function currentLocale(language: string): Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(language)
    ? (language as Locale)
    : "vi";
}

export function JoinCodePanel({ klass }: { klass: Class }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();

  // Deliberately component state, not the query cache.
  const [freshCode, setFreshCode] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<"rotate" | "revoke" | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);

  /**
   * An expired code still occupies the one-active-code slot -- the partial
   * unique index is on `revoked_at IS NULL`, deliberately, because only
   * rotation revokes (§6.1). So the server keeps returning it and the panel has
   * to distinguish "there is a code" from "students can use it".
   *
   * Without this the teacher sees a healthy-looking code with a uses counter
   * while /join turns every student away.
   */
  const [openedAt] = useState(() => Date.now());
  const expired = klass.joinCode
    ? new Date(klass.joinCode.expiresAt).getTime() <= openedAt
    : false;

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["admin-class", klass.id] });

  const rotate = useMutation({
    mutationFn: () => rotateJoinCode(klass.id),
    onSuccess: async (result) => {
      setFreshCode(result.code);
      setConfirming(null);
      setError(null);
      // The previous link is no longer what the button would copy.
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
  const locale = currentLocale(i18n.language);

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
        <div className="px-5 pt-4 pb-3">
          <h2
            id="join-code-heading"
            className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
          >
            {t("classDetail.joinCode")}
          </h2>
        </div>

        <div className="space-y-3 px-5 pb-4">
          {klass.joinCode ? (
            <CodeSummary code={klass.joinCode} expired={expired} locale={locale} />
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
              onClick={() => setConfirming("rotate")}
              disabled={rotate.isPending}
            >
              <RotateCw aria-hidden="true" />
              {klass.joinCode && !expired
                ? t("classDetail.rotate")
                : t("classDetail.issue")}
            </Button>
            {klass.joinCode ? (
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
            {expired && klass.joinCode
              ? t("classDetail.expiredExplainer")
              : t("classDetail.codeShareHint")}
          </p>
        </div>

        <ConfirmDialog
          action={confirming}
          pending={confirming === "revoke" ? revoke.isPending : rotate.isPending}
          onCancel={() => setConfirming(null)}
          onConfirm={() =>
            confirming === "revoke" ? revoke.mutate() : rotate.mutate()
          }
        />

        <FreshCodeDialog
          code={freshCode}
          joinUrl={joinUrl}
          copied={copied}
          onCopy={copyJoinUrl}
          onClose={() => setFreshCode(null)}
        />
      </section>
    </Card>
  );
}

function CodeSummary({
  code,
  expired,
  locale,
}: {
  code: NonNullable<Class["joinCode"]>;
  expired: boolean;
  locale: Locale;
}) {
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
          <dd className="tabular-nums">
            {code.usesCount}
            {" / "}
            {code.maxUses === null ? t("classDetail.unlimited") : code.maxUses}
          </dd>
        </div>
      </dl>
    </>
  );
}

// The deck's "còn 13 ngày": §6.5's 30-day default reads as a safety feature
// only when the teacher can see how much of it is left.
function daysUntil(expiresAt: string): number {
  const ms = new Date(expiresAt).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / 86_400_000));
}

/**
 * The one moment the plaintext code exists. The deck gives it a dialog rather
 * than a corner of the panel, because there is no second chance to read it --
 * dismissing this is the last time anyone sees the code.
 */
function FreshCodeDialog({
  code,
  joinUrl,
  copied,
  onCopy,
  onClose,
}: {
  code: string | null;
  joinUrl: string | null;
  copied: boolean;
  onCopy: (joinUrl: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();

  return (
    <Dialog open={code !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
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

        {joinUrl ? (
          <div className="flex items-center gap-3">
            <QRCodeSVG
              value={joinUrl}
              size={80}
              level="M"
              aria-label={t("classDetail.qrAlt")}
            />
            <div className="min-w-0 space-y-1.5">
              <p className="text-muted-foreground font-mono text-xs break-words">
                {joinUrl}
              </p>
              <Button variant="outline" size="xs" onClick={() => onCopy(joinUrl)}>
                <Copy aria-hidden="true" />
                {copied ? t("classDetail.copied") : t("classDetail.copyLink")}
              </Button>
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

function ConfirmDialog({
  action,
  pending,
  onCancel,
  onConfirm,
}: {
  action: "rotate" | "revoke" | null;
  pending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const { t } = useTranslation();
  const revoking = action === "revoke";

  return (
    <Dialog open={action !== null} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {revoking
              ? t("classDetail.revokeConfirmTitle")
              : t("classDetail.rotateConfirmTitle")}
          </DialogTitle>
          <DialogDescription>
            {revoking
              ? t("classDetail.revokeConfirmBody")
              : t("classDetail.rotateConfirmBody")}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>
            {t("common.cancel")}
          </Button>
          <Button onClick={onConfirm} disabled={pending}>
            {pending
              ? t("common.loading")
              : revoking
                ? t("classDetail.revokeConfirm")
                : t("classDetail.rotateConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
