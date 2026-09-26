import type * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const avatarVariants = cva(
  "inline-grid flex-none place-content-center rounded-full bg-secondary text-secondary-foreground font-semibold",
  {
    variants: {
      size: {
        sm: "size-6 text-[0.625rem]",
        default: "size-8 text-xs",
        lg: "size-10 text-sm",
        "26": "size-6.5 text-2xs",
        "28": "size-7 text-2xs",
        "30": "size-7.5 text-2xs",
        "36": "size-9 text-xs",
        "44": "size-11 text-sm",
        "56": "size-14 text-lg",
      },
      shape: {
        circle: "",
        square: "rounded-lg",
      },
      tone: {
        muted: "",
        self: "bg-brand-soft text-brand-ink",
      },
    },
    compoundVariants: [
      { shape: "square", size: "56", className: "rounded-2xl" },
      { shape: "square", size: ["sm", "26", "28"], className: "rounded-md" },
    ],
    defaultVariants: { size: "default", shape: "circle", tone: "muted" },
  },
);

interface AvatarProps
  extends
    Omit<React.ComponentProps<"span">, "children">,
    VariantProps<typeof avatarVariants> {
  name: string;
}

/**
 * Avatar shows a person's initials, never a photo: there is no avatar upload
 * yet, and a broken image is worse than two letters. It is hidden from
 * assistive technology because the name is always written beside it. `self`
 * marks the signed-in user, in the deck's lime.
 */
export function Avatar({ name, size, shape, tone, className, ...props }: AvatarProps) {
  return (
    <span
      data-slot="avatar"
      aria-hidden="true"
      className={cn(avatarVariants({ size, shape, tone }), className)}
      {...props}
    >
      {initials(name)}
    </span>
  );
}

/**
 * initials takes the given name, then the family name — the deck's rule
 * (docs/design/gaps.md DG-22): "Hoàng Thương" is TH, "Nguyễn Gia Bảo" is BN.
 * A Vietnamese name ends with the given name, which is what people are called.
 */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const family = words.at(0);
  if (family === undefined) return "";
  if (words.length === 1) return family.slice(0, 2).toLocaleUpperCase("vi");
  return (words.at(-1)!.charAt(0) + family.charAt(0)).toLocaleUpperCase("vi");
}
