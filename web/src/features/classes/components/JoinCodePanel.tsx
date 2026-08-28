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
import { formatDate } from "@/lib/i18n/datetime";
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
  const [confirmingRotate, setConfirmingRotate] = useState(false);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["admin-class", klass.id] });

  const rotate = useMutation({
    mutationFn: () => rotateJoinCode(klass.id),
    onSuccess: async (result) => {
      setFreshCode(result.code);
      setConfirmingRotate(false);
      setError(null);
      await invalidate();
    },
    onError: (cause) =>
      setError(
        cause instanceof ApiError ? cause.message : t("classDetail.rotateFailed"),
      ),
  });

  const revoke = useMutation({
    mutationFn: () => revokeJoinCode(klass.id),
    onSuccess: async () => {
      // The old code is gone from the server; keeping it on screen would
      // invite the teacher to hand out something that no longer works.
      setFreshCode(null);
      await invalidate();
    },
    onError: (cause) =>
      setError(
        cause instanceof ApiError ? cause.message : t("classDetail.revokeFailed"),
      ),
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
          <dd>{formatDate(klass.joinCode.expiresAt, currentLocale(i18n.language))}</dd>
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
                  void navigator.clipboard
                    .writeText(joinUrl)
                    .then(() => setCopied(true));
                }}
              >
                {copied ? t("classDetail.copied") : t("classDetail.copyLink")}
              </Button>
            </div>
          ) : null}
        </div>
      ) : klass.joinCode ? (
        <p className="text-muted-foreground mt-4 text-xs leading-relaxed">
          {t("classDetail.rotateToReveal")}
        </p>
      ) : null}

      {error ? (
        <p role="alert" className="text-destructive mt-4 text-sm">
          {error}
        </p>
      ) : null}

      <div className="mt-6 flex flex-wrap gap-2">
        <Button onClick={() => setConfirmingRotate(true)} disabled={rotate.isPending}>
          {klass.joinCode ? t("classDetail.rotate") : t("classDetail.issue")}
        </Button>
        {klass.joinCode ? (
          <Button
            variant="outline"
            onClick={() => revoke.mutate()}
            disabled={revoke.isPending}
          >
            {t("classDetail.disableSelfJoin")}
          </Button>
        ) : null}
      </div>

      {/* §6.4's confirmation. Rotating is not undoable and it takes effect for
          everyone holding the old code, immediately -- including students
          halfway through joining. */}
      <Dialog open={confirmingRotate} onOpenChange={setConfirmingRotate}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("classDetail.rotateConfirmTitle")}</DialogTitle>
            <DialogDescription>{t("classDetail.rotateConfirmBody")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmingRotate(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => rotate.mutate()} disabled={rotate.isPending}>
              {rotate.isPending ? t("common.loading") : t("classDetail.rotateConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
