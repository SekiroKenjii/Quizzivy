import { NavLink } from "react-router";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

const LINK =
  "rounded-seg inline-flex h-7.5 items-center px-3 text-sm font-medium whitespace-nowrap";

function linkClass({ isActive }: Readonly<{ isActive: boolean }>) {
  return cn(
    LINK,
    isActive
      ? "bg-card text-fg shadow-card ring-border ring-1"
      : "text-muted-fg hover:text-fg",
  );
}

/**
 * BankNavigation switches the question bank between standalone questions and
 * question groups, drawn as the deck's segmented track. Each half is a link,
 * so it opens the other page and marks the current one.
 */
export function BankNavigation() {
  const { t } = useTranslation();
  return (
    <nav
      aria-label={t("nav.questionBank")}
      className="bg-muted rounded-ctl inline-flex max-w-full gap-0.5 self-start overflow-x-auto p-[0.1875rem]"
    >
      <NavLink end to="/teacher/question-bank" className={linkClass}>
        {t("groups.standalone")}
      </NavLink>
      <NavLink to="/teacher/question-bank/groups" className={linkClass}>
        {t("groups.bankTitle")}
      </NavLink>
    </nav>
  );
}
