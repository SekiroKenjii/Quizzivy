/**
 * HOME_PILL sizes a Badge to the pill Home draws: 24px high, 9px of padding,
 * 12.5px type and no border, where the deck's status pill elsewhere is 22px.
 */
export const HOME_PILL =
  "border-0 text-meta in-data-[scale=deck]:h-6 in-data-[scale=deck]:px-[9px]";

/**
 * HOME_LIST is the card Coming up and Recent results sit in. It does not clip
 * its rows, so a row's focus ring shows.
 */
export const HOME_LIST = "bg-card shadow-card rounded-2xl border";

/** HOME_ITEM is one entry of a Home list, ruled off from the one above it. */
export const HOME_ITEM = "group/row border-t first:border-t-0";

/**
 * HOME_ROW is what an entry holds: the tile or text, then what trails it. The
 * first and last rows round their own corners to follow the card's.
 */
export const HOME_ROW =
  "flex items-center gap-3.5 px-4 py-3.5 group-first/row:rounded-t-[13px] group-last/row:rounded-b-[13px]";
