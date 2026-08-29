/** Renders a duration as m:ss, the shape a teacher reads off an audio player. */
export function formatDuration(ms: number | null | undefined): string {
  if (ms == null || ms <= 0) return "—";
  const total = Math.round(ms / 1000);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

/** Binary units, matching what an operating system shows for the same file. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${Math.round(kb)} KB`;
  return `${(kb / 1024).toFixed(1)} MB`;
}

/**
 * dd/MM, as the design deck's "Tải lên" column shows it. The year is noise in a
 * library a teacher scans, and the separator is fixed rather than locale-derived
 * because Intl renders "29-08" for `vi` where the deck shows "29/08".
 */
export function formatUploadedAt(iso: string): string {
  const date = new Date(iso);
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}/${month}`;
}
