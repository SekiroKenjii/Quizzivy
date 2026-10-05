import { useState, useRef, useLayoutEffect } from "react";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { FieldControlProps, FormOption } from "./types";
/** SelectField edits one value and dismisses its option list when disabled. */
export function SelectField({
  id,
  label,
  value,
  onChange,
  invalid,
  describedBy,
  disabled,
  options,
  placeholder,
}: FieldControlProps<string> &
  Readonly<{ options: readonly FormOption[]; placeholder?: string | undefined }>) {
  const acceptsChanges = useRef(!disabled);
  useLayoutEffect(() => {
    acceptsChanges.current = !disabled;
    return () => {
      acceptsChanges.current = false;
    };
  }, [disabled]);
  const [open, setOpen] = useState(false);
  if (disabled && open) setOpen(false);
  const selected = options.findIndex((option) => option.value === value);
  return (
    <Select
      open={open && !disabled}
      onOpenChange={(next) => {
        if (!next || acceptsChanges.current) setOpen(next);
      }}
      value={selected < 0 ? "" : String(selected + 1)}
      onValueChange={(next) => {
        if (!acceptsChanges.current) return;
        const option = options[Number(next) - 1];
        if (option) onChange(option.value);
      }}
      disabled={disabled ?? false}
    >
      <SelectTrigger
        id={id}
        aria-label={label}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        className={cn("w-full", invalid && "border-danger")}
      >
        <SelectValue {...(placeholder === undefined ? {} : { placeholder })} />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {options.map((option, index) => (
            <SelectItem
              key={option.value}
              value={String(index + 1)}
              disabled={disabled ?? false}
            >
              {option.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}
