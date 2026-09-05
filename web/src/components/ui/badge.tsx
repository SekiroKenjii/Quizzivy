import type * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

// Matches the design deck's `.badge` (docs/design/mockups/assets/kit.css); an icon inside is 12px (F-14).
const badgeVariants = cva(
  "inline-flex items-center gap-1 rounded-sm border border-transparent px-2 py-px text-xs leading-[1.35] font-medium whitespace-nowrap [&>svg]:shrink-0 [&>svg:not([class*='size-'])]:size-3",
  {
    variants: {
      variant: {
        outline: "border-border text-muted-foreground",
        secondary: "bg-secondary text-secondary-foreground",
        primary: "bg-primary text-primary-foreground",
        // The deck mixes each status colour toward the surface rather than
        // using it flat, so a badge reads as a label and not as an alert.
        success:
          "border-[color-mix(in_oklab,var(--success)_28%,transparent)] bg-[color-mix(in_oklab,var(--success)_12%,var(--background))] text-[color-mix(in_oklab,var(--success)_78%,var(--foreground))]",
        warning:
          "border-[color-mix(in_oklab,var(--warning)_35%,transparent)] bg-[color-mix(in_oklab,var(--warning)_18%,var(--background))] text-[color-mix(in_oklab,var(--warning)_45%,var(--foreground))]",
        danger:
          "border-[color-mix(in_oklab,var(--destructive)_25%,transparent)] bg-[color-mix(in_oklab,var(--destructive)_10%,var(--background))] text-[color-mix(in_oklab,var(--destructive)_82%,var(--foreground))]",
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
