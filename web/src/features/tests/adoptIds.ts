import type { QuestionValues } from "@/features/question-bank/questionSchema";

const LISTS = ["options", "matchItems", "rubric"] as const;

type Row = { id?: string | null };

/**
 * adoptIds gives the rows the builder's form created in this session the ids
 * the server gave them, so an autosave updates those rows rather than
 * replacing them on every save (QC-R6AC-8). The form keeps `id: null` for a
 * row it added; `sent` is the form value the last save sent and `saved` the
 * server's answer in the form's shape. A row is adopted by position, and only
 * while its list has the same length in all three, so a row added or removed
 * since is sent as before and adopted after the next save.
 */
export function adoptIds(
  next: QuestionValues,
  sent: QuestionValues | null,
  saved: QuestionValues | null,
): QuestionValues {
  if (!sent || !saved || sent.type !== next.type) return next;
  let out = next;
  for (const key of LISTS) {
    const rows = next[key] as readonly Row[] | undefined;
    const before = sent[key] as readonly Row[] | undefined;
    const after = saved[key] as readonly Row[] | undefined;
    if (!rows || !before || !after) continue;
    if (rows.length !== before.length || rows.length !== after.length) continue;
    if (!rows.some((row, i) => row.id == null && before[i]?.id == null)) continue;
    out = {
      ...out,
      [key]: rows.map((row, i) =>
        row.id == null && before[i]?.id == null && after[i]?.id
          ? { ...row, id: after[i].id }
          : row,
      ),
    };
  }
  return out;
}
