import { useTranslation } from "react-i18next";
import { studentRules, type RulesDraft } from "@/features/assignments/studentRules";

/** G-01's "Học viên sẽ đọc" panel: the consequences of the switches, in words. */
export function StudentRulesPreview({ draft }: Readonly<{ draft: RulesDraft }>) {
  const { t, i18n } = useTranslation();
  const rules = studentRules(draft, t, i18n.language);

  return (
    <div>
      <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
        {t("assignments.studentWillRead")}
      </p>
      <div className="bg-muted/30 space-y-2 rounded-lg border p-3.5">
        {rules.map((rule) => (
          <p key={rule} className="text-xs leading-relaxed">
            · {rule}
          </p>
        ))}
      </div>
      <p className="text-muted-foreground mt-1.5 text-xs">
        {t("assignments.rulesExact")}
      </p>
    </div>
  );
}
