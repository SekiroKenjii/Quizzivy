const REVOKE_AFTER_MS = 40_000;

/**
 * saveFile hands a downloaded file to the browser to save under `name`,
 * through an object URL. The URL is revoked 40 seconds later, not at once,
 * because Firefox and some Safari versions start the download after the click
 * returns.
 */
export function saveFile(blob: Blob, name: string) {
  const href = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = href;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(href), REVOKE_AFTER_MS);
}
