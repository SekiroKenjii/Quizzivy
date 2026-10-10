import { useTranslation } from "react-i18next";
import { previewRules, type PreviewInput } from "@/features/assignments/studentRules";
import type { Locale } from "@/lib/i18n";

/**
 * StudentRulesPreview is the teacher's "what students will read" panel: the
 * sentences the student's intro will show for the switches and dates on the
 * form, from the same generator (`previewRules`).
 */
export function StudentRulesPreview({ draft }: Readonly<{ draft: PreviewInput }>) {
  const { t, i18n } = useTranslation();
  const rules = previewRules(draft, t, i18n.language as Locale, new Date());

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
