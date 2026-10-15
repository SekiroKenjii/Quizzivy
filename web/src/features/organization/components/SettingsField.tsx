import type { ReactNode } from "react";
import { Label } from "@/components/ui/label";

/**
 * SettingsField is a settings form's field: its label, the control, and
 * under it the error if there is one, else the hint. A field is at most
 * `--field-max` wide. Name the message ids `{id}-error` and `{id}-hint` in
 * the control's `aria-describedby`, as `describedBy` does.
 */
export function SettingsField({
  id,
  label,
  hint,
  error,
  children,
}: Readonly<{
  id: string;
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
  children: ReactNode;
}>) {
  return (
    <div className="flex max-w-(--field-max) min-w-0 flex-col gap-1.5">
      <Label
        htmlFor={id}
        className="text-meta in-data-[scale=deck]:text-meta leading-normal font-medium"
      >
        {label}
      </Label>
      {children}
      {error === undefined ? null : (
        <p id={`${id}-error`} className="text-danger-ink text-xs leading-normal">
          {error}
        </p>
      )}
      {error === undefined && hint !== undefined ? (
        <p id={`${id}-hint`} className="text-muted-fg text-xs leading-normal">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
