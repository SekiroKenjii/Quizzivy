import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Input is a single-line text field. `size` picks the deck's larger fields
 * (`lg` 42px, `xl` 46px on the front door) inside a `data-scale="deck"`
 * surface; elsewhere every size renders today's field.
 */
function Input({
  className,
  type,
  size,
  ...props
}: Omit<React.ComponentProps<"input">, "size"> & { size?: "default" | "lg" | "xl" }) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "border-input selection:bg-primary selection:text-primary-foreground file:text-foreground placeholder:text-muted-foreground dark:bg-input/30 h-9 w-full min-w-0 rounded-md border bg-transparent px-3 py-1 text-[length:var(--text-input)] shadow-xs transition-[color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 lg:text-sm",
        "focus-visible:border-ring",
        "aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40",
        "in-data-[scale=deck]:lg:text-ui in-data-[scale=deck]:aria-invalid:border-danger in-data-[scale=deck]:h-9.5 in-data-[scale=deck]:px-3 in-data-[scale=deck]:shadow-none in-data-[scale=deck]:focus-visible:ring-0",
        size === "lg" &&
          "in-data-[scale=deck]:rounded-ctl in-data-[scale=deck]:lg:text-body in-data-[scale=deck]:h-10.5",
        size === "xl" &&
          "in-data-[scale=deck]:lg:text-md in-data-[scale=deck]:h-11.5 in-data-[scale=deck]:rounded-lg in-data-[scale=deck]:px-3.5",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
