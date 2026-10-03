import { useTranslation } from "react-i18next";
import type { IntegrityPolicy, ReviewPolicy } from "@/features/assignments/api";
import { studentRules } from "@/features/assignments/studentRules";
import type { Locale } from "@/lib/i18n";
import { fromDateTimeInput } from "@/lib/i18n/datetime";

/**
 * StudentRulesPreview is the teacher's "what students will read" panel: the
 * sentences the student's intro will show for the switches and dates on the
 * form, from the same generator. A date that is not filled in yet leaves the
 * availability sentence out.
 */
export function StudentRulesPreview({
  draft,
}: Readonly<{
  draft: Readonly<{
    review: ReviewPolicy;
    integrity: IntegrityPolicy;
    opensAt: string;
    closesAt: string;
  }>;
}>) {
  const { t, i18n } = useTranslation();
  const now = new Date();
  const opensAt = fromDateTimeInput(draft.opensAt);
  const closesAt = fromDateTimeInput(draft.closesAt);
  const dated = !Number.isNaN(opensAt.getTime()) && !Number.isNaN(closesAt.getTime());
  const rules = studentRules(
    {
      review: draft.review,
      integrity: draft.integrity,
      ...(dated
        ? { window: { opensAt, closesAt, upcoming: opensAt.getTime() > now.getTime() } }
        : {}),
    },
    t,
    i18n.language as Locale,
    now,
  );

  return (
    <div>
      <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
        {t("assignments.studentWillRead")}
      </p>
      <div className="bg-muted/30 space-y-2 rounded-lg border p-3.5">
        {rules.map((rule) => (
          <p key={rule.id} className="text-xs leading-relaxed">
            · {rule.text}
          </p>
        ))}
      </div>
      <p className="text-muted-foreground mt-1.5 text-xs">
        {t("assignments.rulesExact")}
      </p>
    </div>
  );
}
