import type * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  [
    "inline-flex items-center border border-transparent text-xs leading-[1.35] font-medium whitespace-nowrap [&>svg]:shrink-0 [&>svg:not([class*='size-'])]:size-3",
    "h-5.5 gap-1.5 rounded-full px-2.5 py-0",
  ],
  {
    variants: {
      variant: {
        outline: ["border-border text-muted-foreground", "rounded-sm"],
        secondary: "bg-secondary text-secondary-foreground",
        primary: "bg-primary text-primary-foreground",
        // The deck mixes each status colour toward the surface rather than
        // using it flat, so a badge reads as a label and not as an alert.
        success: ["text-success-ink", "bg-success-soft border-transparent"],
        warning: "bg-warning-soft text-warning-ink border-transparent",
        danger: "bg-danger-soft text-danger-ink border-transparent",
        info: "bg-info-soft text-info-ink",
        brand: "bg-brand-soft text-brand-ink",
        count:
          "h-5 min-w-5 justify-center rounded-full bg-muted px-1.5 text-2xs font-semibold text-muted-fg",
        "count-brand":
          "h-5 min-w-5 justify-center rounded-full bg-brand px-1.5 text-2xs font-semibold text-brand-fg",
      },
    },
    defaultVariants: { variant: "outline" },
  },
);

/**
 * Badge is a label: a status pill, a count, or a role. The status variants are
 * the deck's soft pills, and `dot` adds the 6px dot the deck draws before a
 * status.
 */
function Badge({
  className,
  variant,
  dot = false,
  children,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { dot?: boolean }) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    >
      {dot && (
        <span
          aria-hidden="true"
          className="size-1.5 shrink-0 rounded-full bg-current"
        />
      )}
      {children}
    </span>
  );
}

export { Badge, badgeVariants };
