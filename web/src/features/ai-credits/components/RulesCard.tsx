import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CirclePause } from "lucide-react";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Segmented } from "@/components/ui/segmented";
import type { AiRules, AiRulesInput } from "@/features/ai-credits/api";
import { withOption } from "@/features/ai-credits/figures";
import { useSaveRules } from "@/features/ai-credits/mutations";
import { credits } from "@/features/ai/credits";
import { failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";

const ALLOWANCES = [20_000, 50_000, 100_000] as const;
const WARNINGS = [80, 90] as const;

function Rule({
  label,
  hint,
  children,
}: Readonly<{ label: string; hint: string; children: ReactNode }>) {
  return (
    <div className="flex flex-col gap-2 border-t py-3.5 first:border-t-0">
      <div>
        <p className="text-ui leading-normal font-medium">{label}</p>
        <p className="text-muted-fg text-meta leading-normal text-pretty">{hint}</p>
      </div>
      {children}
    </div>
  );
}

/**
 * RulesCard holds what applies to every teacher without a custom allowance:
 * the monthly allowance, when to warn the teacher, and what happens when a
 * teacher runs out. A choice is saved as it is made, with one request at a
 * time; until the answer comes the chosen option is shown, and a refusal puts
 * the one in force back. What happens when the centre pool runs out has one
 * value, pausing AI for everyone, and is a fixed line: "Add credits" is how
 * an Admin approves more (DG-196). A figure the maintenance command set that
 * the buttons do not offer is shown as an option too.
 */
export function RulesCard({ rules }: Readonly<{ rules: AiRules }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const save = useSaveRules();

  function choose(change: AiRulesInput) {
    if (save.isPending) return;
    save.mutate(change, {
      onSuccess: () => notify.success(t("aiCredits.rules.saved")),
      onError: (cause) =>
        notify.error(failureMessage(cause, t("aiCredits.rules.failed"))),
    });
  }

  const asked: AiRulesInput = save.isPending ? save.variables : {};
  const allowance = asked.defaultAllowance ?? rules.defaultAllowance;
  const warn = asked.warnPercent ?? rules.warnPercent;
  const limit = asked.onTeacherLimit ?? rules.onTeacherLimit;

  return (
    <SettingsCard
      title={t("aiCredits.rules.title")}
      description={t("aiCredits.rules.hint")}
      headerClassName="px-4 py-3.5"
    >
      <div className="flex flex-col px-4 pb-1">
        <Rule
          label={t("aiCredits.rules.allowance.label")}
          hint={t("aiCredits.rules.allowance.hint")}
        >
          <Segmented
            label={t("aiCredits.rules.allowance.label")}
            value={String(allowance)}
            pending={save.isPending}
            className="max-w-full flex-wrap self-start"
            options={withOption(ALLOWANCES, rules.defaultAllowance).map((value) => ({
              value: String(value),
              label: credits(value, locale),
            }))}
            onChange={(value) => choose({ defaultAllowance: Number(value) })}
          />
        </Rule>
        <Rule
          label={t("aiCredits.rules.warn.label")}
          hint={t("aiCredits.rules.warn.hint")}
        >
          <Segmented
            label={t("aiCredits.rules.warn.label")}
            value={String(warn)}
            pending={save.isPending}
            className="max-w-full flex-wrap self-start"
            options={withOption(WARNINGS, rules.warnPercent).map((value) => ({
              value: String(value),
              label: `${value}%`,
            }))}
            onChange={(value) => choose({ warnPercent: Number(value) })}
          />
        </Rule>
        <Rule
          label={t("aiCredits.rules.limit.label")}
          hint={t("aiCredits.rules.limit.hint")}
        >
          <Segmented
            label={t("aiCredits.rules.limit.label")}
            value={limit}
            pending={save.isPending}
            className="max-w-full flex-wrap self-start"
            options={[
              { value: "pause", label: t("aiCredits.rules.limit.pause") },
              { value: "request", label: t("aiCredits.rules.limit.request") },
            ]}
            onChange={(value) =>
              choose({ onTeacherLimit: value === "pause" ? "pause" : "request" })
            }
          />
        </Rule>
        <Rule
          label={t("aiCredits.rules.pool.label")}
          hint={t("aiCredits.rules.pool.hint")}
        >
          <p className="flex items-center gap-1.5 text-sm font-medium">
            <CirclePause
              aria-hidden="true"
              className="text-muted-fg size-3.75 flex-none"
            />
            {t("aiCredits.rules.pool.value")}
          </p>
        </Rule>
      </div>
    </SettingsCard>
  );
}
