/** 24-hour, as the deck writes it ("Đã lưu 09:41"). */
export function hhmm(iso: string): string {
  return new Date(iso).toLocaleTimeString("vi-VN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

/** Day and month only ("03/09"): the year is never in doubt on a test paper. */
export function ddmm(iso: string): string {
  const date = new Date(iso);
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}`;
}

const pad = (n: number) => String(n).padStart(2, "0");
