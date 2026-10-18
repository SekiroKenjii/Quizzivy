import {
  PAGE_MAX,
  SEARCH_MAX,
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
 * searchOf is the text the list sends as `q`: what was typed in NFC without
 * the space around it, cut to SEARCH_MAX characters, or nothing while it is
 * shorter than SEARCH_MIN characters.
 */
export function searchOf(typed: string): string {
  const letters = [...typed.normalize("NFC").trim()];
  if (letters.length < SEARCH_MIN) return "";
  return letters.slice(0, SEARCH_MAX).join("").trim();
}

/** pageOf is the page the list asks for: the URL's page, no higher than the contract takes. */
export function pageOf(page: number): number {
  return Math.min(page, PAGE_MAX);
}

/** isTooShort reports whether something was typed that is not yet long enough to search. */
export function isTooShort(typed: string): boolean {
  const text = typed.trim();
  return text !== "" && lengthOf(text) < SEARCH_MIN;
}
