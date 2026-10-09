import type { TFunction } from "i18next";

/** Binary units, matching what an operating system shows for the same file. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${Math.round(kb)} KB`;
  return `${(kb / 1024).toFixed(1)} MB`;
}

const GIB = 1024 * 1024 * 1024;
const NO_BREAK_SPACE = String.fromCharCode(0xa0);

/**
 * formatStorage is formatBytes with a gigabyte step, for a library's usage and
 * quota, as the deck writes them: from 0.1 GB it shows gigabytes with up to
 * two decimals and drops trailing zeros, so a 5120 MiB quota reads "5 GB" and
 * 200 MiB "0.2 GB". A no-break space keeps the number with its unit when the
 * line wraps.
 */
export function formatStorage(bytes: number): string {
  if (bytes < GIB / 10) return formatBytes(bytes).replace(" ", NO_BREAK_SPACE);
  return `${Number((bytes / GIB).toFixed(2))}${NO_BREAK_SPACE}GB`;
}

/** playLimitLabel names a default play limit as the card and the upload dialog say it; 0 is unlimited. */
export function playLimitLabel(plays: number, t: TFunction): string {
  return plays === 0 ? t("media.playsUnlimited") : t("media.plays", { count: plays });
}
