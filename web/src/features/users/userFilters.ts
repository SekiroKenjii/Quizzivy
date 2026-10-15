import {
  SEARCH_MIN,
  USER_PAGE_SIZES,
  USER_STATES,
  USER_TABS,
  type AdminUserTab,
  type UserPageSize,
  type UserStateFilter,
} from "@/features/users/api";

/** tabOf is the tab a URL names, All for an absent or unknown one. */
export function tabOf(raw: string | null): AdminUserTab {
  return USER_TABS.find((tab) => tab === raw) ?? "all";
}

/** stateOf is the state a URL names, none for an absent or unknown one. */
export function stateOf(raw: string | null): UserStateFilter | null {
  return USER_STATES.find((state) => state === raw) ?? null;
}

/** limitOf is the page size the list asks for, the default when `size` is not one it offers. */
export function limitOf(size: number): UserPageSize {
  return USER_PAGE_SIZES.find((option) => option === size) ?? USER_PAGE_SIZES[0];
}

function lengthOf(text: string): number {
  return [...text.normalize("NFC")].length;
}

/**
 * searchOf is the text the list sends as `q`: what was typed without the
 * space around it, or nothing while it is shorter than SEARCH_MIN characters.
 */
export function searchOf(typed: string): string {
  const text = typed.trim();
  return lengthOf(text) >= SEARCH_MIN ? text : "";
}

/** isTooShort reports whether something was typed that is not yet long enough to search. */
export function isTooShort(typed: string): boolean {
  const text = typed.trim();
  return text !== "" && lengthOf(text) < SEARCH_MIN;
}
