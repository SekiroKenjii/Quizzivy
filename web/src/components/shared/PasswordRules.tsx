import { useTranslation } from "react-i18next";
import { Circle, CircleAlert, CircleCheck } from "lucide-react";

import { passwordRules, passwordStrength } from "@/lib/password";
import { cn } from "@/lib/utils";

const BARS = [0, 1, 2, 3];

/**
 * PasswordRules is the deck's strength meter and checklist under a new
 * password field. `revealed` turns the unmet rules into errors once the user
 * has tried to save. The third rule, a different password from the one it
 * replaces, only the server can check: it stays a stated rule until `refusal`
 * carries the server's reason. `id` names the checklist for the field's
 * `aria-describedby`.
 */
export function PasswordRules({
  id,
  password,
  revealed,
  refusal,
  className,
}: Readonly<{
  id: string;
  password: string;
  revealed: boolean;
  refusal: string | null;
  className?: string;
}>) {
  const { t } = useTranslation();
  const rules = passwordRules(password);
  const score = passwordStrength(password);
  const fill = fillFor(score);
  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <div aria-hidden="true" data-testid="password-meter" className="flex gap-1">
        {BARS.map((bar) => (
          <span
            key={bar}
            data-filled={bar < score || undefined}
            className={cn("h-1 flex-1 rounded-[4px]", bar < score ? fill : "bg-muted")}
          />
        ))}
      </div>
      <ul id={id} className="flex flex-col gap-1.5">
        <Rule
          state={ruleState(rules.length, revealed)}
          label={t("changePassword.rules.length")}
        />
        <Rule
          state={ruleState(rules.numberOrSymbol, revealed)}
          label={t("changePassword.rules.numberOrSymbol")}
        />
        {refusal ? (
          <Rule state="failed" label={refusal} />
        ) : (
          <Rule state="stated" label={t("changePassword.rules.different")} />
        )}
      </ul>
    </div>
  );
}

type RuleState = "met" | "unmet" | "failed" | "stated";

const RULE = {
  met: {
    icon: CircleCheck,
    tone: "text-success-ink",
    status: "changePassword.rules.met",
  },
  unmet: { icon: Circle, tone: "text-muted-fg", status: "changePassword.rules.unmet" },
  failed: {
    icon: CircleAlert,
    tone: "text-danger-ink",
    status: "changePassword.rules.unmet",
  },
  stated: { icon: Circle, tone: "text-muted-fg", status: null },
} as const;

function Rule({ state, label }: Readonly<{ state: RuleState; label: string }>) {
  const { t } = useTranslation();
  const { icon: Icon, tone, status } = RULE[state];
  return (
    <li data-state={state} className={cn("flex items-center gap-2 text-sm", tone)}>
      <Icon aria-hidden="true" className="size-3.5 shrink-0" />
      <span>{label}</span>
      {status && <span className="sr-only">· {t(status)}</span>}
    </li>
  );
}

function ruleState(met: boolean, revealed: boolean): RuleState {
  if (met) return "met";
  return revealed ? "failed" : "unmet";
}

function fillFor(score: number) {
  if (score <= 1) return "bg-danger";
  return score === 2 ? "bg-warning" : "bg-success";
}
