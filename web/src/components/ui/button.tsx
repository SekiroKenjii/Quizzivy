import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { Slot } from "radix-ui";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        destructive: "bg-danger-solid text-danger-solid-fg hover:bg-danger-solid/90",
        outline: [
          "border shadow-xs hover:text-accent-foreground",
          "bg-card hover:bg-muted dark:border-border dark:bg-card dark:hover:bg-muted",
        ],
        secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/80",
        ghost: "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: [
          "h-9 py-2",
          "px-3.5 text-ui gap-1.5 [&_svg:not([class*='size-'])]:size-[15px]",
        ],
        xs: [
          "gap-1 rounded-sm px-2 text-xs [&_svg:not([class*='size-'])]:size-3.5",
          "h-6.5",
        ],
        sm: [
          "gap-1.5 px-3",
          "h-7.5 rounded-seg text-sm [&_svg:not([class*='size-'])]:size-3.5",
        ],
        md: "h-9.5 gap-1.5 rounded-ctl px-3.5 text-ui [&_svg:not([class*='size-'])]:size-[15px]",
        lg: "h-10.5 rounded-lg px-4 text-base font-semibold",
        xl: "h-11.5 gap-2 rounded-lg px-4 text-md font-semibold",
        icon: "size-8.5",
        "icon-xs": ["rounded-sm [&_svg:not([class*='size-'])]:size-3.5", "size-6"],
        "icon-sm": "size-7 rounded-seg",
        "icon-lg": "size-9",
        "icon-xl": "size-10 rounded-lg",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot.Root : "button";

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
