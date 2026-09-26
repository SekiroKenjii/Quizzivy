import type * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  [
    "inline-flex items-center gap-1 rounded-sm border border-transparent px-2 py-px text-xs leading-[1.35] font-medium whitespace-nowrap [&>svg]:shrink-0 [&>svg:not([class*='size-'])]:size-3",
    "in-data-[scale=deck]:h-5.5 in-data-[scale=deck]:gap-1.5 in-data-[scale=deck]:rounded-full in-data-[scale=deck]:px-2.5 in-data-[scale=deck]:py-0",
  ],
  {
    variants: {
      variant: {
        outline: [
          "border-border text-muted-foreground",
          "in-data-[scale=deck]:rounded-sm",
        ],
        secondary: "bg-secondary text-secondary-foreground",
        primary: "bg-primary text-primary-foreground",
        // The deck mixes each status colour toward the surface rather than
        // using it flat, so a badge reads as a label and not as an alert.
        success: [
          "border-[color-mix(in_oklab,var(--success)_28%,transparent)] bg-[color-mix(in_oklab,var(--success)_12%,var(--background))] text-[color-mix(in_oklab,var(--success)_78%,var(--foreground))]",
          "in-data-[scale=deck]:bg-success-soft in-data-[scale=deck]:text-success-ink in-data-[scale=deck]:border-transparent",
        ],
        warning: [
          "border-[color-mix(in_oklab,var(--warning)_35%,transparent)] bg-[color-mix(in_oklab,var(--warning)_18%,var(--background))] text-[color-mix(in_oklab,var(--warning)_45%,var(--foreground))]",
          "in-data-[scale=deck]:bg-warning-soft in-data-[scale=deck]:text-warning-ink in-data-[scale=deck]:border-transparent",
        ],
        danger: [
          "border-[color-mix(in_oklab,var(--destructive)_25%,transparent)] bg-[color-mix(in_oklab,var(--destructive)_10%,var(--background))] text-[color-mix(in_oklab,var(--destructive)_82%,var(--foreground))]",
          "in-data-[scale=deck]:bg-danger-soft in-data-[scale=deck]:text-danger-ink in-data-[scale=deck]:border-transparent",
        ],
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
 * Badge is a label: a status pill, a count, or a role. On a deck surface the
 * status variants become the deck's soft pills, and `dot` adds the 6px dot the
 * deck draws before a status.
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
