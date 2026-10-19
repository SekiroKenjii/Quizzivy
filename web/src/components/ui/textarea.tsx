import type * as React from "react";

import { cn } from "@/lib/utils";

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "border-input placeholder:text-muted-foreground focus-visible:border-ring aria-invalid:ring-destructive/20 flex min-h-20 w-full resize-y rounded-md border bg-transparent py-2 text-[length:var(--text-input)] leading-relaxed transition-[color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50",
        "lg:text-ui aria-invalid:border-danger px-3 shadow-none focus-visible:ring-0",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
