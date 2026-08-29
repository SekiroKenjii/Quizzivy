import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/components/ui/button";
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
    <section className="rounded-lg border p-6" aria-labelledby="join-code-heading">
      <h2 id="join-code-heading" className="text-base font-semibold">
        {t("classDetail.joinCode")}
      </h2>

      {klass.joinCode ? (
        <CodeSummary code={klass.joinCode} expired={expired} locale={locale} />
      ) : (
        <p className="text-muted-foreground mt-4 text-sm">
          {t("classDetail.noActiveCode")}
        </p>
      )}

      {freshCode ? (
        <FreshCode
          code={freshCode}
          joinUrl={joinUrl}
          copied={copied}
          onCopy={copyJoinUrl}
        />
      ) : klass.joinCode ? (
        <p className="text-muted-foreground mt-4 text-xs leading-relaxed">
          {expired
            ? t("classDetail.expiredExplainer")
            : t("classDetail.rotateToReveal")}
        </p>
      ) : null}

      {error ? (
        <p role="alert" className="text-destructive mt-4 text-sm">
          {error}
        </p>
      ) : null}

      <div className="mt-6 flex flex-wrap gap-2">
        <Button onClick={() => setConfirming("rotate")} disabled={rotate.isPending}>
          {klass.joinCode && !expired
            ? t("classDetail.rotate")
            : t("classDetail.issue")}
        </Button>
        {klass.joinCode ? (
          <Button
            variant="outline"
            onClick={() => setConfirming("revoke")}
            disabled={revoke.isPending}
          >
            {t("classDetail.disableSelfJoin")}
          </Button>
        ) : null}
      </div>

      <ConfirmDialog
        action={confirming}
        pending={confirming === "revoke" ? revoke.isPending : rotate.isPending}
        onCancel={() => setConfirming(null)}
        onConfirm={() => (confirming === "revoke" ? revoke.mutate() : rotate.mutate())}
      />
    </section>
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

  return (
    <dl className="mt-4 grid grid-cols-2 gap-y-3 text-sm">
      <dt className="text-muted-foreground">{t("classDetail.codeHint")}</dt>
      <dd className="font-mono">{t("classDetail.maskedCode", { hint: code.hint })}</dd>
      <dt className="text-muted-foreground">{t("classDetail.expiresAt")}</dt>
      <dd>
        {formatDateTime(code.expiresAt, locale)}
        {expired ? (
          <span className="text-destructive ml-2 text-xs font-medium">
            {t("classDetail.expiredBadge")}
          </span>
        ) : null}
      </dd>
      <dt className="text-muted-foreground">{t("classDetail.uses")}</dt>
      <dd>
        {code.usesCount}
        {code.maxUses === null ? "" : ` / ${code.maxUses}`}
      </dd>
    </dl>
  );
}

function FreshCode({
  code,
  joinUrl,
  copied,
  onCopy,
}: {
  code: string;
  joinUrl: string | null;
  copied: boolean;
  onCopy: (joinUrl: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="mt-6 rounded-md border p-4 text-center">
      <p className="text-muted-foreground text-xs">{t("classDetail.shownOnce")}</p>
      <p className="mt-2 font-mono text-2xl tracking-widest">{code}</p>
      {joinUrl ? (
        <div className="mt-4 flex flex-col items-center gap-3">
          <QRCodeSVG
            value={joinUrl}
            size={144}
            level="M"
            aria-label={t("classDetail.qrAlt")}
          />
          <Button variant="outline" size="sm" onClick={() => onCopy(joinUrl)}>
            {copied ? t("classDetail.copied") : t("classDetail.copyLink")}
          </Button>
        </div>
      ) : null}
    </div>
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
