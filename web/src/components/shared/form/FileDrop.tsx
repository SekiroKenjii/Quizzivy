import { useRef } from "react";
import { useTranslation } from "react-i18next";
import { FileCheck, Upload } from "lucide-react";
import { useFileDrop } from "@/hooks/useFileDrop";
import { cn } from "@/lib/utils";

/** FileDrop chooses or removes one file without validating or uploading it. */
export function FileDrop({
  id,
  value,
  onChange,
  accept,
  placeholder,
  limits,
  invalid = false,
  disabled = false,
  ...props
}: Readonly<{
  id?: string | undefined;
  value: File | null;
  onChange: (file: File | null) => void;
  accept?: string | undefined;
  placeholder?: string | undefined;
  limits?: string | undefined;
  invalid?: boolean | undefined;
  disabled?: boolean | undefined;
  "aria-describedby"?: string | undefined;
}>) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const dragging = useFileDrop((files) => {
    if (files[0]) onChange(files[0]);
  }, !disabled);
  const Icon = value === null ? Upload : FileCheck;
  return (
    <div aria-invalid={invalid || undefined} className="contents">
      <button
        {...props}
        id={id}
        type="button"
        disabled={disabled}
        aria-label={
          value === null
            ? undefined
            : t("formDialog.fileRemoveNamed", { name: value.name })
        }
        className={cn(
          "border-border flex w-full flex-col items-center gap-1.5 rounded-lg border-[1.5px] border-dashed px-3.5 py-5 text-center disabled:opacity-45",
          (dragging || value !== null) && "bg-muted",
          dragging && "border-primary",
          invalid && "border-danger",
        )}
        onClick={() => {
          if (value !== null) {
            onChange(null);
            if (input.current) input.current.value = "";
          } else input.current?.click();
        }}
      >
        <Icon
          aria-hidden="true"
          className={cn(
            "size-5.5",
            value === null ? "text-muted-fg" : "text-success-ink",
          )}
        />
        <span className="text-ui font-medium">
          {value?.name ?? placeholder ?? t("formDialog.fileChoose")}
        </span>
        <span className="text-muted-fg text-xs">
          {value === null ? limits : t("formDialog.fileRemove")}
        </span>
      </button>
      <input
        ref={input}
        type="file"
        accept={accept}
        disabled={disabled}
        hidden
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          if (file) onChange(file);
          event.currentTarget.value = "";
        }}
      />
    </div>
  );
}
