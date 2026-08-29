import type * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

// The deck's `.avatar`: initials on the secondary surface, never a photo. There
// is no avatar upload in v1 and a broken image is worse than two letters.
const avatarVariants = cva(
  "inline-grid flex-none place-content-center rounded-full bg-secondary text-secondary-foreground font-semibold",
  {
    variants: {
      size: {
        sm: "size-6 text-[0.625rem]",
        default: "size-8 text-xs",
        lg: "size-10 text-sm",
      },
    },
    defaultVariants: { size: "default" },
  },
);

interface AvatarProps
  extends
    Omit<React.ComponentProps<"span">, "children">,
    VariantProps<typeof avatarVariants> {
  name: string;
}

export function Avatar({ name, size, className, ...props }: AvatarProps) {
  return (
    <span
      data-slot="avatar"
      aria-hidden="true"
      className={cn(avatarVariants({ size }), className)}
      {...props}
    >
      {initials(name)}
    </span>
  );
}

/**
 * Family name then given name, which is the deck's rule: "Nguyễn Đức Minh" is
 * NM. Vietnamese middle names are the least distinguishing part of a name, so
 * taking the first and last words keeps both halves a teacher actually uses.
 */
function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const first = words.at(0);
  if (first === undefined) return "";
  if (words.length === 1) return first.slice(0, 2).toLocaleUpperCase("vi");
  return (first.charAt(0) + words.at(-1)!.charAt(0)).toLocaleUpperCase("vi");
}
