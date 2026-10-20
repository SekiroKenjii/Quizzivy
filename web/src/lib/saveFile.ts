import type { DownloadedFile } from "@/lib/api/client";

const REVOKE_AFTER_MS = 40_000;

/**
 * saveDownloadedFile hands a downloaded file to the browser to save, under the
 * name the server gave it or else `fallbackName`, through an object URL. The
 * URL is revoked 40 seconds later, not at once, because Firefox and some
 * Safari versions start the download after the click returns.
 */
export function saveDownloadedFile(file: DownloadedFile, fallbackName: string) {
  const href = URL.createObjectURL(file.blob);
  const link = document.createElement("a");
  link.href = href;
  link.download = file.filename ?? fallbackName;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(href), REVOKE_AFTER_MS);
}
