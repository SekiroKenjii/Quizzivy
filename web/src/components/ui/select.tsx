import type * as React from "react";

import { cn } from "@/lib/utils";

// A native <select>, matching the deck's `.select` (kit.css) including its
// inline chevron. Native because the deck's usages are short, single-choice
// lists where the platform control is better on a touch device than a
// re-implemented listbox.
function Select({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <select
      data-slot="select"
      className={cn(
        "border-input focus-visible:border-ring focus-visible:ring-ring/50 bg-[url('data:image/svg+xml;utf8,<svg xmlns=%27http://www.w3.org/2000/svg%27 viewBox=%270 0 24 24%27 fill=%27none%27 stroke=%27%2371717a%27 stroke-width=%272%27 stroke-linecap=%27round%27 stroke-linejoin=%27round%27><path d=%27m6 9 6 6 6-6%27/></svg>')] min-h-9 w-full appearance-none rounded-md border bg-transparent bg-[length:1rem] bg-[right_0.625rem_center] bg-no-repeat py-1.5 pr-8 pl-3 text-sm shadow-xs transition-[color,box-shadow] outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}

export { Select };
