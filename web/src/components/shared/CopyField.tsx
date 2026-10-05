import { useId, useRef } from "react";

import { CopyButton } from "@/components/shared/CopyButton";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const ACTION =
  "border-border bg-card hover:bg-muted dark:hover:bg-muted rounded-seg h-7 border px-2.5 in-data-[scale=deck]:h-7";

/**
 * CopyField shows a value nobody edits, such as a join link or a code, under
 * its label, with a button beside it that copies the value. When the
 * clipboard refuses, the whole value is selected and takes focus, so the next
 * copy shortcut takes it by hand, and `failedMessage` says so under the
 * field. `copyLabel` replaces "Copy" on the button, and `mono` sets the value
 * in the monospace face.
 */
export function CopyField({
  label,
  value,
  failedMessage,
  copyLabel,
  mono = false,
}: Readonly<{
  label: string;
  value: string;
  failedMessage: string;
  copyLabel?: string | undefined;
  mono?: boolean | undefined;
}>) {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-meta leading-normal font-medium">
        {label}
      </label>
      <div className="flex flex-wrap items-center gap-2 [&>[role=alert]]:text-left">
        <Input
          ref={input}
          id={id}
          readOnly
          value={value}
          className={cn("w-auto flex-[1_1_10rem]", mono && "font-mono")}
        />
        <CopyButton
          value={value}
          failedMessage={failedMessage}
          label={copyLabel}
          className={ACTION}
          onFailed={() => {
            input.current?.focus();
            input.current?.select();
          }}
        />
      </div>
    </div>
  );
}
