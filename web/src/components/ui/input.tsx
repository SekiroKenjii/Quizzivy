import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Input is a single-line text field. `size` picks the deck's larger fields:
 * `lg` 42px, `xl` 46px on the front door.
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
        "border-input selection:bg-primary selection:text-primary-foreground file:text-foreground placeholder:text-muted-foreground w-full min-w-0 rounded-md border py-1 text-[length:var(--text-input)] transition-[color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        "focus-visible:border-ring",
        "aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40",
        "bg-bg dark:bg-bg lg:text-ui aria-invalid:border-danger h-9.5 px-3 shadow-none focus-visible:ring-0",
        size === "lg" && "rounded-ctl lg:text-body h-10.5",
        size === "xl" && "lg:text-md h-11.5 rounded-lg px-3.5",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
