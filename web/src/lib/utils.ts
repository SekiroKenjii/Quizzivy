import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** Merge Tailwind classes, last-wins on conflicts. Used by every shadcn primitive. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
