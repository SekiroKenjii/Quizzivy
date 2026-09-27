import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const twMerge = extendTailwindMerge({
  extend: {
    theme: {
      text: [
        "3xs",
        "2xs",
        "caption",
        "meta",
        "ui",
        "body",
        "md",
        "title",
        "stat-sm",
        "stat",
        "h1",
        "h1-student",
        "kpi-sm",
        "kpi",
        "display",
        "input",
      ],
      radius: ["seg", "ctl"],
      shadow: ["card", "float"],
    },
  },
});

/**
 * Merge Tailwind classes, last-wins on conflicts. Used by every shadcn
 * primitive. It knows the deck's own font sizes, radii and shadows from
 * index.css, so `text-ui` replaces `text-sm` and never a text colour.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
