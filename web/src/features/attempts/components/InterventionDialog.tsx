import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { ApiError } from "@/lib/api/errors";
import { extendAttempt, resetAttempt, voidAttempt, type MonitorRow } from "../api";

export type Intervention = "extend" | "reset" | "void";

const EXTENSIONS = [5, 10, 15, 20, 30, 45, 60];

/**
 * G-02b: extend, reset and void are one dialog shape learned once -- what will
 * happen, one required reason, two buttons. The confirming button stays
 * disabled until the reason is typed, because each of these changes a record
 * a parent may ask about weeks later.
 */
export function InterventionDialog({
  kind,
  row,
  onOpenChange,
  onDone,
}: {
  kind: Intervention | null;
  row: MonitorRow | null;
  onOpenChange: (open: boolean) => void;
  onDone: () => Promise<void> | void;
}) {
  const { t } = useTranslation();
  const [minutes, setMinutes] = useState(10);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);

  const act = useMutation({
    mutationFn: async () => {
      if (row?.attemptId == null || kind === null) return;
      const trimmed = reason.trim();
      if (kind === "extend")
        await extendAttempt(row.attemptId, { minutes, reason: trimmed });
      if (kind === "reset") await resetAttempt(row.attemptId, { reason: trimmed });
      if (kind === "void") await voidAttempt(row.attemptId, { reason: trimmed });
    },
    onSuccess: async () => {
      setReason("");
      setError(null);
      onOpenChange(false);
      await onDone();
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("monitor.actionFailed")),
  });

  if (kind === null || row === null) return null;
  const name = row.fullName;
  const no = row.attemptNo ?? 1;
  const blank = reason.trim() === "";

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open) {
          setReason("");
          setError(null);
        }
        onOpenChange(open);
      }}
      title={t(`monitor.${kind}.title`, { name, no })}
      description={t(`monitor.${kind}.body`, { name, no, next: no + 1 })}
      confirmLabel={t(`monitor.${kind}.confirm`)}
      destructive={kind === "void"}
      disabled={blank}
      pending={act.isPending}
      error={error}
      onConfirm={() => act.mutate()}
    >
      <div className="space-y-3">
        {kind === "extend" && (
          <div className="space-y-1.5">
            <Label htmlFor="intervention-minutes">{t("monitor.extend.minutes")}</Label>
            <Select
              value={String(minutes)}
              onValueChange={(next) => setMinutes(Number(next))}
            >
              <SelectTrigger id="intervention-minutes" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {EXTENSIONS.map((m) => (
                  <SelectItem key={m} value={String(m)}>
                    {t("assignments.minutes", { count: m })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        <div className="space-y-1.5">
          <Label htmlFor="intervention-reason">{t("monitor.reason")}</Label>
          {kind === "extend" ? (
            <Input
              id="intervention-reason"
              value={reason}
              placeholder={t("monitor.extend.placeholder")}
              onChange={(event) => setReason(event.target.value)}
            />
          ) : (
            <Textarea
              id="intervention-reason"
              value={reason}
              placeholder={t(`monitor.${kind}.placeholder`)}
              onChange={(event) => setReason(event.target.value)}
            />
          )}
          <p className="text-muted-foreground text-xs">{t("monitor.reasonHint")}</p>
        </div>
      </div>
    </ConfirmDialog>
  );
}
