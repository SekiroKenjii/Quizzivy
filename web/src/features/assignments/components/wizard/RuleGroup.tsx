import { useId, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";

/**
 * RuleGroup is the deck's bordered group of wizard settings: a heading row on
 * the sidebar tone with its icon, then the rows, each divided by a rule.
 */
export function RuleGroup({
  icon: Icon,
  title,
  children,
}: Readonly<{ icon: LucideIcon; title: string; children: ReactNode }>) {
  const id = useId();
  return (
    <section
      aria-labelledby={id}
      className="border-border overflow-hidden rounded-[10px] border [&>*+*]:border-t"
    >
      <h3
        id={id}
        className="bg-sidebar flex items-center gap-2 px-3.5 py-2.5 text-sm font-semibold"
      >
        <Icon aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
        {title}
      </h3>
      {children}
    </section>
  );
}

/**
 * RuleText is a row's name and the line under it, which says what the
 * setting does. `id` names the line for the control it describes.
 */
export function RuleText({
  label,
  hint,
  id,
  labelFor,
}: Readonly<{ label: string; hint: string; id?: string; labelFor?: string }>) {
  return (
    <span className="min-w-0">
      {labelFor === undefined ? (
        <span className="text-ui block leading-normal font-medium">{label}</span>
      ) : (
        <label htmlFor={labelFor} className="text-ui block leading-normal font-medium">
          {label}
        </label>
      )}
      <span id={id} className="text-muted-fg text-meta block leading-normal">
        {hint}
      </span>
    </span>
  );
}

/**
 * SwitchRow is a setting turned on or off: its name, what it does, and the
 * switch. A disabled row is drawn at 45% and cannot be changed.
 */
export function SwitchRow({
  label,
  hint,
  checked,
  onChange,
  disabled = false,
}: Readonly<{
  label: string;
  hint: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}>) {
  const id = useId();
  return (
    <div
      className={cn("flex items-center gap-3.5 px-3.5 py-3", disabled && "opacity-45")}
    >
      <span className="min-w-0 flex-1">
        <RuleText label={label} hint={hint} id={`${id}-hint`} labelFor={id} />
      </span>
      <Switch
        id={id}
        checked={checked}
        disabled={disabled}
        aria-describedby={`${id}-hint`}
        onCheckedChange={onChange}
      />
    </div>
  );
}

/**
 * RuleRow is a setting with a control of its own under its text, or beside
 * it when `inline`.
 */
export function RuleRow({
  label,
  hint,
  inline = false,
  dimmed = false,
  children,
}: Readonly<{
  label: string;
  hint: string;
  inline?: boolean;
  dimmed?: boolean;
  children: ReactNode;
}>) {
  return (
    <div
      className={cn(
        "px-3.5 py-3",
        inline
          ? "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1.5"
          : "flex flex-col gap-2",
        dimmed && "opacity-45",
      )}
    >
      <RuleText label={label} hint={hint} />
      <span className={cn(inline && "justify-self-end")}>{children}</span>
    </div>
  );
}
