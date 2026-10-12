import { useEffect, useId, useRef, useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import {
  CircleAlert,
  CircleCheck,
  CircleX,
  FileText,
  KeyRound,
  Upload,
  X,
} from "lucide-react";
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
  const { t } = useTranslation();
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
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 text-sm font-medium">
        <label htmlFor={id}>{label}</label>
        <span className="text-muted-fg text-[11.5px] font-medium">
          {required ? t("imports.upload.required") : t("imports.upload.optional")}
        </span>
      </div>
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
          "flex flex-wrap items-center rounded-[10px] transition-colors duration-150",
          name === null
            ? "gap-3.5 border-[1.5px] border-dashed px-4 py-3.5"
            : "bg-card gap-3 border px-3 py-2.5",
          over && "border-primary bg-muted",
          error !== null && "border-danger",
          slot.phase === "rejected" && "bg-danger-soft",
        )}
      >
        <SlotTile role={role} filled={name !== null} />
        {name === null ? (
          <>
            <span className="min-w-0 flex-[1_1_200px]">
              <span className="text-ui block leading-normal">
                {over ? t("imports.upload.dropActive") : t("imports.upload.dropText")}
              </span>
              <span className="text-muted-fg block text-xs leading-normal">
                {t(`imports.upload.${role}Hint`)}
              </span>
            </span>
            <Button
              ref={chooseButton}
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              aria-label={t(`imports.upload.${role}Choose`)}
              className="h-[34px] rounded-md in-data-[scale=deck]:h-[34px] in-data-[scale=deck]:rounded-md"
              onClick={() => input.current?.click()}
            >
              <Upload aria-hidden="true" />
              {t("imports.upload.choose")}
            </Button>
          </>
        ) : (
          <>
            <FileDetails name={name} bytes={bytes} slot={slot} />
            <span className="flex items-center gap-1">
              <Button
                ref={replaceButton}
                type="button"
                variant="outline"
                size="sm"
                disabled={disabled}
                aria-label={t("imports.upload.replaceNamed", { name })}
                className="text-meta in-data-[scale=deck]:text-meta h-8 px-2.5 in-data-[scale=deck]:h-8"
                onClick={() => input.current?.click()}
              >
                {t("imports.upload.replace")}
              </Button>
              {slot.file === null ? null : (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  disabled={disabled}
                  aria-label={t("imports.upload.removeNamed", { name })}
                  className="text-muted-fg hover:text-fg size-8 in-data-[scale=deck]:size-8"
                  onClick={() => {
                    setDropError(null);
                    refocus.current = true;
                    onRemove();
                  }}
                >
                  <X aria-hidden="true" className="size-[15px]" />
                </Button>
              )}
            </span>
          </>
        )}
      </div>
      {error === null ? null : (
        <p role="alert" className="text-danger-ink flex items-start gap-1.5 text-xs">
          <CircleAlert className="mt-px size-3.5 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}
    </div>
  );
}

function SlotTile({
  role,
  filled,
}: Readonly<{ role: ImportSourceRole; filled: boolean }>) {
  const Icon = role === "exam" ? FileText : KeyRound;
  return (
    <span
      className={cn(
        "bg-muted grid shrink-0 place-items-center",
        filled ? "size-[34px] rounded-[8px]" : "size-9 rounded-[9px]",
      )}
    >
      <Icon aria-hidden="true" className={filled ? "size-4" : "size-[17px]"} />
    </span>
  );
}

function FileDetails({
  name,
  bytes,
  slot,
}: Readonly<{ name: string; bytes: number | null; slot: SourceSlot }>) {
  const { t, i18n } = useTranslation();
  return (
    <span className="min-w-0 flex-[1_1_220px]">
      <span className="text-ui block truncate leading-normal font-medium" title={name}>
        {name}
      </span>
      <span className="text-muted-fg flex flex-wrap items-center gap-1.5 text-xs leading-normal">
        <SlotPhase slot={slot} />
        {bytes === null ? null : <span>· {formatBytes(bytes)}</span>}
      </span>
      {slot.phase === "uploading" ? (
        <span className="mt-1 flex items-center gap-3">
          <progress
            className="h-2 flex-1"
            value={slot.fraction}
            max={1}
            aria-label={t("imports.upload.uploadingNamed", { name })}
          />
          <span className="text-muted-fg text-xs tabular-nums">
            {percent(i18n.language, slot.fraction)}
          </span>
        </span>
      ) : null}
    </span>
  );
}

function SlotPhase({ slot }: Readonly<{ slot: SourceSlot }>) {
  const { t } = useTranslation();
  if (slot.phase === "uploaded")
    return (
      <span className="text-success-ink inline-flex items-center gap-1.5">
        <CircleCheck aria-hidden="true" className="size-[13px]" />
        {t("imports.upload.received")}
      </span>
    );
  if (slot.phase === "uploading")
    return <span role="status">{t("imports.upload.uploading")}</span>;
  if (slot.phase === "rejected")
    return (
      <span className="text-danger-ink inline-flex items-center gap-1.5">
        <CircleX aria-hidden="true" className="size-[13px]" />
        {t("imports.upload.notAccepted")}
      </span>
    );
  return <span>{t("imports.upload.notSent")}</span>;
}
