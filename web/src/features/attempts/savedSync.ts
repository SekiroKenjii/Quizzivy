import { useState } from "react";

/**
 * useSavedSync keeps a grading form honest about the grade the server holds
 * when that grade can change under it, from another tab or another teacher.
 * `saved` and `local` are signatures of the server's grade and the form's.
 * While the form still shows the grade it started from, a new server grade
 * replaces it through `reset`. Once the teacher has changed the form, a new
 * server grade is left alone and reported as `elsewhere`, so the form can
 * say that saving replaces it. `dirty` is a change the teacher has not saved.
 * It sets state during render and converges: each write makes `base` equal
 * `saved`, which ends the condition.
 */
export function useSavedSync(saved: string, local: string, reset: () => void) {
  const [base, setBase] = useState(saved);
  if (saved !== base && (local === base || local === saved)) {
    if (local === base) reset();
    setBase(saved);
    return { dirty: false, elsewhere: false };
  }
  return { dirty: local !== base, elsewhere: saved !== base };
}
