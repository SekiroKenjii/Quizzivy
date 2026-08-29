import type * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

// Matches the design deck's `.badge` (docs/design/mockups/assets/kit.css).
const badgeVariants = cva(
  "inline-flex items-center gap-1 rounded-sm border border-transparent px-2 py-px text-xs leading-[1.35] font-medium whitespace-nowrap",
  {
    variants: {
      variant: {
        outline: "border-border text-muted-foreground",
        secondary: "bg-secondary text-secondary-foreground",
      },
    },
    defaultVariants: { variant: "outline" },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
