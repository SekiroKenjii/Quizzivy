import type { TFunction } from "i18next";
import { saveError } from "@/features/classes/classFields";
import { ApiError } from "@/lib/api/errors";

/**
 * teacherRefusal is what a dialog says when the server refuses the teacher it
 * was given (`INVALID_RECIPIENT`), else what `saveError` says.
 */
export function teacherRefusal(cause: unknown, t: TFunction, fallback: string): string {
  if (cause instanceof ApiError && cause.code === "INVALID_RECIPIENT")
    return t("classes.admin.invalidTeacher");
  return saveError(cause, fallback);
}
