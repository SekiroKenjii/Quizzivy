import type { AiAllowanceInput, AiTeacherMonth } from "@/features/ai-credits/api";

/** MIN_ALLOWANCE is the least a custom monthly allowance may be, in credits. */
export const MIN_ALLOWANCE = 1000;
/** MAX_ALLOWANCE is the most a custom monthly allowance may be, in credits. */
export const MAX_ALLOWANCE = 10_000_000;

/** AllowanceValues are the allowance dialog's fields: which allowance, its figure as typed, and the pause. */
export type AllowanceValues = {
  mode: "default" | "custom";
  credits: string;
  paused: boolean;
};

/**
 * baseAllowance is a custom teacher's own figure: the allowance less the
 * credits approved requests added on top of it, which a new figure leaves.
 */
export function baseAllowance(teacher: AiTeacherMonth): number {
  return Math.max(teacher.allowance - teacher.granted, 0);
}

/**
 * allowanceBody is what the Admin changed in the dialog, as the body of
 * `PUT /admin/ai/teachers/{userId}/allowance`, or null when nothing differs
 * from the teacher's month. A figure is sent only when it moved, and a
 * teacher returned to the default is sent no figure.
 */
export function allowanceBody(
  teacher: AiTeacherMonth,
  values: AllowanceValues,
): AiAllowanceInput | null {
  const body: AiAllowanceInput = {};
  if (values.mode === "default" && teacher.custom) body.custom = false;
  if (values.mode === "custom") {
    const figure = Number(values.credits.trim());
    if (!teacher.custom || figure !== baseAllowance(teacher)) {
      body.custom = true;
      body.credits = figure;
    }
  }
  if (values.paused !== teacher.paused) body.paused = values.paused;
  return Object.keys(body).length === 0 ? null : body;
}
