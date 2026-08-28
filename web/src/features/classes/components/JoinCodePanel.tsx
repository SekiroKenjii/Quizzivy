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
  // Captured once rather than read during render: `Date.now()` in a render body
  // is impure, and a value that changes on every re-render for no reason is
  // exactly what that rule exists to stop. A code does not expire while the
  // teacher looks at the panel, and a refetch remounts this with fresh data.
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
      // Close the dialog as well as reporting. The error renders in the
      // panel, and Radix marks everything outside an open dialog
      // `aria-hidden` -- so leaving it open puts the explanation behind a
      // dialog that now looks like it simply does nothing, and takes it
      // out of the accessibility tree entirely.
      setConfirming(null);
      setError(
        cause instanceof ApiError ? cause.message : t("classDetail.rotateFailed"),
      );
    },
  });

  const revoke = useMutation({
    mutationFn: () => revokeJoinCode(klass.id),
    onSuccess: async () => {
      // The old code is gone from the server; keeping it on screen would
      // invite the teacher to hand out something that no longer works.
      setFreshCode(null);
      setConfirming(null);
      setCopied(false);
      // Rotate cleared this and revoke did not, so a failed rotate followed by
      // a successful revoke left a red error describing an operation two steps
      // back.
      setError(null);
      await invalidate();
    },
    onError: (cause) => {
      // Close the dialog as well as reporting. The error renders in the
      // panel, and Radix marks everything outside an open dialog
      // `aria-hidden` -- so leaving it open puts the explanation behind a
      // dialog that now looks like it simply does nothing, and takes it
      // out of the accessibility tree entirely.
      setConfirming(null);
      setError(
        cause instanceof ApiError ? cause.message : t("classDetail.revokeFailed"),
      );
    },
  });

  const joinUrl = freshCode
    ? `${window.location.origin}/join/${freshCode.replace("-", "")}`
    : null;

  return (
    <section className="rounded-lg border p-6" aria-labelledby="join-code-heading">
      <h2 id="join-code-heading" className="text-base font-semibold">
        {t("classDetail.joinCode")}
      </h2>

      {klass.joinCode ? (
        <dl className="mt-4 grid grid-cols-2 gap-y-3 text-sm">
          <dt className="text-muted-foreground">{t("classDetail.codeHint")}</dt>
          <dd className="font-mono">
            {t("classDetail.maskedCode", { hint: klass.joinCode.hint })}
          </dd>
          <dt className="text-muted-foreground">{t("classDetail.expiresAt")}</dt>
          {/* formatDateTime, not formatDate: this is a bearer secret's expiry,
              and dd/MM/yyyy renders a code that died at 09:00 identically to
              one good until 23:59. */}
          <dd>
            {formatDateTime(klass.joinCode.expiresAt, currentLocale(i18n.language))}
            {expired ? (
              <span className="text-destructive ml-2 text-xs font-medium">
                {t("classDetail.expiredBadge")}
              </span>
            ) : null}
          </dd>
          <dt className="text-muted-foreground">{t("classDetail.uses")}</dt>
          <dd>
            {klass.joinCode.usesCount}
            {klass.joinCode.maxUses === null ? "" : ` / ${klass.joinCode.maxUses}`}
          </dd>
        </dl>
      ) : (
        <p className="text-muted-foreground mt-4 text-sm">
          {t("classDetail.noActiveCode")}
        </p>
      )}

      {freshCode ? (
        <div className="mt-6 rounded-md border p-4 text-center">
          <p className="text-muted-foreground text-xs">{t("classDetail.shownOnce")}</p>
          <p className="mt-2 font-mono text-2xl tracking-widest">{freshCode}</p>
          {joinUrl ? (
            <div className="mt-4 flex flex-col items-center gap-3">
              {/* Encodes the join LINK, not the bare code: a phone camera
                  opens it, which is the whole point of a QR on a whiteboard. */}
              <QRCodeSVG
                value={joinUrl}
                size={144}
                level="M"
                aria-label={t("classDetail.qrAlt")}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  // Both halves fail, and neither is exotic. On a non-secure
                  // origin `navigator.clipboard` is undefined, so reading
                  // .writeText throws synchronously; with the permission denied
                  // the promise rejects. Unguarded, each one gives a button
                  // that does nothing and says nothing -- and the teacher's
                  // fallback (select the link and copy it by hand) is only
                  // obvious once someone says so.
                  const clipboard = navigator.clipboard as Clipboard | undefined;
                  if (!clipboard) {
                    setCopied(false);
                    setError(t("classDetail.copyFailed"));
                    return;
                  }
                  void clipboard.writeText(joinUrl).then(
                    () => {
                      setError(null);
                      setCopied(true);
                    },
                    () => {
                      setCopied(false);
                      setError(t("classDetail.copyFailed"));
                    },
                  );
                }}
              >
                {copied ? t("classDetail.copied") : t("classDetail.copyLink")}
              </Button>
            </div>
          ) : null}
        </div>
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
          {/* An expired code cannot be rotated INTO anything a student is
              already using, so the honest verb is "issue". */}
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

      {/* §6.4's confirmation. Both actions get one, because both share the
          property that earned rotate its dialog: not undoable, and effective
          immediately for everyone holding the old code -- including a student
          halfway through joining. Revoking is if anything the harsher of the
          two, since it issues no replacement. */}
      <Dialog
        open={confirming !== null}
        onOpenChange={(open) => !open && setConfirming(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {confirming === "revoke"
                ? t("classDetail.revokeConfirmTitle")
                : t("classDetail.rotateConfirmTitle")}
            </DialogTitle>
            <DialogDescription>
              {confirming === "revoke"
                ? t("classDetail.revokeConfirmBody")
                : t("classDetail.rotateConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(null)}>
              {t("common.cancel")}
            </Button>
            {confirming === "revoke" ? (
              <Button onClick={() => revoke.mutate()} disabled={revoke.isPending}>
                {revoke.isPending
                  ? t("common.loading")
                  : t("classDetail.revokeConfirm")}
              </Button>
            ) : (
              <Button onClick={() => rotate.mutate()} disabled={rotate.isPending}>
                {rotate.isPending
                  ? t("common.loading")
                  : t("classDetail.rotateConfirm")}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
