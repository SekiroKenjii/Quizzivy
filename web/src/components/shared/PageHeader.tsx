import { Button } from "@/components/ui/button";
import { ArrowLeft } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

interface PageHeaderProps {
  title: string;
  meta?: ReactNode;
  actions?: ReactNode;
  backTo?: string;
}

/**
 * The contextual bar the deck puts on every detail screen (G-01, G-02, G-06):
 * what you are looking at, and what you can do to it.
 *
 * The deck draws it *replacing* the global topbar. It sits below it instead,
 * because the global bar carries the command palette and the account menu, and
 * chrome that vanishes on half the screens is chrome nobody relies on — on the
 * deck's own G-01 there is no way to reach settings or sign out. Everything the
 * deck's version carries is here; only the 3.5rem is new.
 */
export function PageHeader({ title, meta, actions, backTo }: PageHeaderProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <header className="bg-background sticky z-10 -mx-6 -mt-6 mb-6 border-b">
      <div className="flex h-14 items-center gap-3 px-6">
        {backTo === undefined ? null : (
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("common.back")}
            onClick={() => void navigate(backTo)}
          >
            <ArrowLeft aria-hidden="true" />
          </Button>
        )}
        <h1 className="truncate text-sm font-medium">{title}</h1>
        {meta}
        {actions === undefined ? null : (
          <div className="ml-auto flex items-center gap-2">{actions}</div>
        )}
      </div>
    </header>
  );
}
