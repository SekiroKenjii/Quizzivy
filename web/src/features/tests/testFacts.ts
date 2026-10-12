import type { TFunction } from "i18next";
import type { Test } from "@/features/tests/api";

/**
 * assignedLabel names how a test is in use, as its card and its detail show
 * it: the live assignments when there are any, else that one is scheduled,
 * else the closed ones, else that it is not assigned.
 */
export function assignedLabel(test: Pick<Test, "assignments">, t: TFunction): string {
  const { live, scheduled, closed } = test.assignments;
  if (live > 0) return t("tests.assigned.live", { count: live });
  if (scheduled > 0) return t("tests.assigned.scheduled");
  if (closed > 0) return t("tests.assigned.closed", { count: closed });
  return t("tests.assigned.none");
}
