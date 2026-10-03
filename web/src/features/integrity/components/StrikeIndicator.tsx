import { useTranslation } from "react-i18next";
import type { StrikeState } from "../strikes";

/**
 * StrikeIndicator is the engine's standing count of the times away the
 * student has left, which spec §10.2 asks for whenever the assignment sets a
 * limit; the deck draws none, so it stays one phrase in the save line's row.
 * While some are left it is a plain number in the row's muted ink. A spent
 * allowance reads in the full ink, and past it the warning ink says what the
 * dialog said: under `flag` that the teacher has been told, otherwise that the
 * limit is passed. It reads the same `StrikeState` as the dialog, so the two
 * never disagree, and it renders nothing when there is no limit.
 */
export function StrikeIndicator({ state }: Readonly<{ state: StrikeState }>) {
  const { t } = useTranslation();
  if (state.limit === null || state.remaining === null) return null;

  if (state.exceeded) {
    return (
      <span className="text-warning-ink font-medium">
        {t(state.consequence === "flag" ? "integrity.flagged" : "integrity.overLimit")}
      </span>
    );
  }
  if (state.remaining === 0) {
    return <span className="text-fg font-medium">{t("integrity.spent")}</span>;
  }
  return <span>{t("integrity.remaining", { count: state.remaining })}</span>;
}
