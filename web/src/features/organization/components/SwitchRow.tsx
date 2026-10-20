import { Switch } from "@/components/ui/switch";

const PAUSED =
  "aria-disabled:data-[state=checked]:bg-primary/50 aria-disabled:data-[state=unchecked]:bg-border/50 aria-disabled:[&>span]:bg-switch-thumb/50";

/**
 * SwitchRow is a row of a settings card: a label and a hint on the left, a
 * switch at the end. The card draws the lines between rows. While `paused` the switch stays focusable but answers
 * `aria-disabled` and ignores a press, and fades its colours, not the element.
 */
export function SwitchRow({
  id,
  label,
  hint,
  checked,
  paused = false,
  onChange,
}: Readonly<{
  id: string;
  label: string;
  hint: string;
  checked: boolean;
  paused?: boolean | undefined;
  onChange: (on: boolean) => void;
}>) {
  return (
    <div className="flex items-center gap-3.5 px-4.5 py-3">
      <span className="min-w-0 flex-1">
        <span id={`${id}-label`} className="text-ui block leading-normal font-medium">
          {label}
        </span>
        <span
          id={`${id}-hint`}
          className="text-muted-fg text-meta block leading-normal"
        >
          {hint}
        </span>
      </span>
      <Switch
        id={id}
        checked={checked}
        aria-labelledby={`${id}-label`}
        aria-describedby={`${id}-hint`}
        aria-disabled={paused || undefined}
        className={PAUSED}
        onCheckedChange={(on) => {
          if (!paused) onChange(on);
        }}
      />
    </div>
  );
}
