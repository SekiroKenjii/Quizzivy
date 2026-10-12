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
/**
 * SelectField edits one value and dismisses its option list when disabled.
 * While `locked` (its form is saving) the trigger answers `aria-disabled`,
 * keeps focus and opens nothing, and a change is ignored (T-35).
 */
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
  locked = false,
}: FieldControlProps<string> &
  Readonly<{
    options: readonly FormOption[];
    placeholder?: string | undefined;
    locked?: boolean | undefined;
  }>) {
  const still = (disabled ?? false) || locked;
  const acceptsChanges = useRef(!still);
  useLayoutEffect(() => {
    acceptsChanges.current = !still;
    return () => {
      acceptsChanges.current = false;
    };
  }, [still]);
  const [open, setOpen] = useState(false);
  if (still && open) setOpen(false);
  const selected = options.findIndex((option) => option.value === value);
  return (
    <Select
      open={open && !still}
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
        aria-disabled={locked || undefined}
        className={cn(
          "w-full",
          invalid && "border-danger",
          locked && "cursor-default opacity-50",
        )}
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
