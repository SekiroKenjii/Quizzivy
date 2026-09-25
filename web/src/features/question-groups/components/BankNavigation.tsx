import { NavLink } from "react-router";
import { useTranslation } from "react-i18next";
import { buttonVariants } from "@/components/ui/button";

export function BankNavigation() {
  const { t } = useTranslation();
  return (
    <nav
      aria-label={t("nav.questionBank")}
      className="flex flex-wrap gap-2 border-b pb-3"
    >
      <NavLink
        end
        to="/admin/question-bank"
        className={({ isActive }) =>
          buttonVariants({ variant: isActive ? "secondary" : "ghost", size: "sm" })
        }
      >
        {t("groups.standalone")}
      </NavLink>
      <NavLink
        to="/admin/question-bank/groups"
        className={({ isActive }) =>
          buttonVariants({ variant: isActive ? "secondary" : "ghost", size: "sm" })
        }
      >
        {t("groups.bankTitle")}
      </NavLink>
    </nav>
  );
}
