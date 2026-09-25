import { useEffect, useId, useRef, useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, CircleCheck, FileText, Upload } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatBytes } from "@/features/media/format";
import { cn } from "@/lib/utils";
import type { ImportSourceRole } from "../api";
import type { SourceSlot } from "../useSourceIntake";

function percent(locale: string, fraction: number): string {
  return new Intl.NumberFormat(locale, {
    style: "percent",
    maximumFractionDigits: 0,
  }).format(fraction);
}

/**
 * FileSlot is one labelled source file: a native file picker, an equivalent
 * drop target, and the file's own validation and upload state beside it.
 * Choosing or removing a file keeps keyboard focus in the slot.
 */
export function FileSlot({
  role,
  slot,
  accept,
  required,
  disabled,
  onChoose,
  onRemove,
}: Readonly<{
  role: ImportSourceRole;
  slot: SourceSlot;
  accept: string;
  required: boolean;
  disabled: boolean;
  onChoose: (file: File) => void;
  onRemove: () => void;
}>) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const chooseButton = useRef<HTMLButtonElement>(null);
  const replaceButton = useRef<HTMLButtonElement>(null);
  const refocus = useRef(false);

  useEffect(() => {
    if (!refocus.current) return;
    refocus.current = false;
    (replaceButton.current ?? chooseButton.current)?.focus();
  });
  const [over, setOver] = useState(false);
  const [dropError, setDropError] = useState<string | null>(null);
  const label = t(`imports.upload.${role}Label`);

  const onDragOver = (event: DragEvent<HTMLDivElement>) => {
    if (disabled || !event.dataTransfer.types.includes("Files")) return;
    event.preventDefault();
    setOver(true);
  };
  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    if (disabled || !event.dataTransfer.types.includes("Files")) return;
    event.preventDefault();
    setOver(false);
    const dropped = [...event.dataTransfer.files];
    if (dropped.length !== 1) {
      setDropError(t("imports.upload.dropOne"));
      return;
    }
    setDropError(null);
    refocus.current = true;
    onChoose(dropped[0]!);
  };

  const name = slot.file?.name ?? slot.received?.filename ?? null;
  const bytes = slot.file?.size ?? slot.received?.bytes ?? null;
  const error = dropError ?? slot.error;

  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between gap-3">
        <label htmlFor={id} className="text-sm font-medium">
          {label}
        </label>
        <span className="text-muted-foreground text-xs">
          {required ? t("imports.upload.required") : t("imports.upload.optional")}
        </span>
      </div>
      <p className="text-muted-foreground text-xs leading-relaxed">
        {t(`imports.upload.${role}Hint`)}
      </p>
      <input
        ref={input}
        id={id}
        type="file"
        tabIndex={-1}
        accept={accept}
        className="sr-only"
        disabled={disabled}
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) {
            setDropError(null);
            refocus.current = true;
            onChoose(file);
          }
          event.target.value = "";
        }}
      />
      <div
        role="group"
        aria-label={t(`imports.upload.${role}Zone`)}
        data-testid={`drop-${role}`}
        onDragOver={onDragOver}
        onDragLeave={() => setOver(false)}
        onDrop={onDrop}
        className={cn(
          "rounded-lg border border-dashed p-4 transition-colors duration-150",
          over && "border-foreground/40 bg-secondary",
          error !== null && "border-destructive/40",
        )}
      >
        {name === null ? (
          <div className="flex flex-wrap items-center gap-3">
            <Button
              ref={chooseButton}
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              aria-label={t(`imports.upload.${role}Choose`)}
              onClick={() => input.current?.click()}
            >
              <Upload aria-hidden="true" />
              {t("imports.upload.choose")}
            </Button>
            <span className="text-muted-foreground text-xs">
              {t("imports.upload.orDrop")}
            </span>
          </div>
        ) : (
          <div className="flex flex-wrap items-start gap-3">
            <FileText
              className="text-muted-foreground mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
            <div className="min-w-0 flex-1 space-y-1">
              <p className="text-sm font-medium break-all">{name}</p>
              <p className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
                {bytes === null ? null : <span>{formatBytes(bytes)}</span>}
                <SlotPhase slot={slot} />
              </p>
              {slot.phase === "uploading" ? (
                <div className="flex items-center gap-3">
                  <progress
                    className="h-2 flex-1"
                    value={slot.fraction}
                    max={1}
                    aria-label={t("imports.upload.uploadingNamed", { name })}
                  />
                  <span className="text-muted-foreground text-xs tabular-nums">
                    {percent(i18n.language, slot.fraction)}
                  </span>
                </div>
              ) : null}
            </div>
            <div className="flex items-center gap-2">
              <Button
                ref={replaceButton}
                type="button"
                variant="outline"
                size="xs"
                disabled={disabled}
                aria-label={t("imports.upload.replaceNamed", { name })}
                onClick={() => input.current?.click()}
              >
                {t("imports.upload.replace")}
              </Button>
              {slot.file === null ? null : (
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  disabled={disabled}
                  aria-label={t("imports.upload.removeNamed", { name })}
                  onClick={() => {
                    setDropError(null);
                    refocus.current = true;
                    onRemove();
                  }}
                >
                  {t("imports.upload.remove")}
                </Button>
              )}
            </div>
          </div>
        )}
      </div>
      {error === null ? null : (
        <p role="alert" className="text-destructive flex items-start gap-1.5 text-xs">
          <CircleAlert className="mt-px size-3.5 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}
    </div>
  );
}

function SlotPhase({ slot }: Readonly<{ slot: SourceSlot }>) {
  const { t } = useTranslation();
  if (slot.phase === "uploaded")
    return (
      <Badge variant="outline">
        <CircleCheck aria-hidden="true" />
        {t("imports.upload.received")}
      </Badge>
    );
  if (slot.phase === "uploading")
    return <span role="status">{t("imports.upload.uploading")}</span>;
  if (slot.phase === "rejected") return <span>{t("imports.upload.notAccepted")}</span>;
  return <span>{t("imports.upload.notSent")}</span>;
}
