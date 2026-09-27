import type * as React from "react";

import { cn } from "@/lib/utils";

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "border-input placeholder:text-muted-foreground focus-visible:border-ring aria-invalid:border-destructive aria-invalid:ring-destructive/20 flex min-h-20 w-full resize-y rounded-md border bg-transparent px-3 py-2 text-[length:var(--text-input)] leading-relaxed shadow-xs transition-[color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50 lg:text-sm",
        "in-data-[scale=deck]:lg:text-ui in-data-[scale=deck]:aria-invalid:border-danger in-data-[scale=deck]:px-3 in-data-[scale=deck]:shadow-none in-data-[scale=deck]:focus-visible:ring-0",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
