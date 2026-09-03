import { useTranslation } from "react-i18next";
import type { StrikeState } from "../strikes";

/**
 * "Còn 2 lần rời trang" -- a number, not a warning. Present tense, no icon,
 * no colour, until the allowance is actually spent (S-05). Alarm styling here
 * would punish the student who alt-tabbed to a calculator once.
 */
export function StrikeIndicator({ state }: { state: StrikeState }) {
  const { t } = useTranslation();
  if (state.limit === null || state.remaining === null) return null;

  if (state.exceeded) {
    return (
      <span className="text-foreground font-medium">
        {t(state.consequence === "flag" ? "integrity.flagged" : "integrity.overLimit")}
      </span>
    );
  }
  if (state.remaining === 0) {
    return <span className="text-foreground font-medium">{t("integrity.spent")}</span>;
  }
  return <span>{t("integrity.remaining", { count: state.remaining })}</span>;
}
