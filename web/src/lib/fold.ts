/** Accent-insensitive comparison key, so "nghe" finds "nghé" and "d" finds "đ" (§13.8). */
export function fold(text: string): string {
  return text
    .toLocaleLowerCase("vi")
    .normalize("NFD")
    .replaceAll(/[̀-ͯ]/g, "")
    .replaceAll("đ", "d");
}
