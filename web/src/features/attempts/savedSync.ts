import { useState, type FocusEvent } from "react";

/**
 * useSavedSync keeps a grading form honest about the grade the server holds
 * when that grade can change under it, from another tab or another teacher.
 * `saved` and `local` are signatures of the server's grade and the form's.
 * While the teacher has not touched the form, a new server grade replaces
 * it through `reset`. A form the teacher is in (`holding`, its focus) or has
 * changed since its last sync is touched, even if the edit happens to equal
 * the starting grade again: a new server grade is left alone and reported as
 * `elsewhere`, so the form can say that saving replaces it. `dirty` is a
 * change the teacher has not saved.
 * It sets state during render and converges: each write makes `base` equal
 * `saved`, or `touched` true, which ends its condition.
 */
export function useSavedSync(
  saved: string,
  local: string,
  reset: () => void,
  holding = false,
) {
  const [base, setBase] = useState(saved);
  const [touched, setTouched] = useState(false);
  if (saved !== base && (local === saved || (local === base && !touched && !holding))) {
    if (local !== saved) reset();
    setBase(saved);
    setTouched(false);
    return { dirty: false, elsewhere: false };
  }
  if (local !== base && !touched) setTouched(true);
  return { dirty: local !== base, elsewhere: saved !== base };
}

/**
 * pointsSignature is a points box's text as the number it saves, so "0.50"
 * and the server's 0.5 read as one grade. Text that is not a number stays
 * as typed.
 */
export function pointsSignature(text: string): string {
  const trimmed = text.trim();
  const value = Number(trimmed);
  return trimmed !== "" && Number.isFinite(value) ? String(value) : trimmed;
}

/**
 * useFocusWithin says whether focus is inside an element, for the `holding`
 * of useSavedSync: spread `bind` on the element.
 */
export function useFocusWithin() {
  const [within, setWithin] = useState(false);
  const bind = {
    onFocus: () => setWithin(true),
    onBlur: (event: FocusEvent<HTMLElement>) => {
      if (!event.currentTarget.contains(event.relatedTarget)) setWithin(false);
    },
  };
  return [within, bind] as const;
}
