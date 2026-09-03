import { useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageBarSlot } from "@/layouts/pageBar";

interface BarProps {
  variant?: "bar";
  title: string;
  meta?: ReactNode;
  actions?: ReactNode;
  backTo?: string;
}

interface TitleProps {
  variant: "title";
  title: string;
  subtitle?: ReactNode;
  actions?: ReactNode;
}

/**
 * The one header every admin screen goes through, in the deck's two shapes.
 *
 * `bar` is the contextual bar of the detail screens (G-01, G-02, G-06): back,
 * what you are looking at, and what you can do to it. The deck keeps the
 * global topbar and sets this bar under it, so it is rendered into the
 * shell's slot between the topbar and <main> -- outside the scroll container,
 * where it stays put the way the topbar does, with no sticky and no negative
 * margins reaching out of the padding. Without a shell (tests, other layouts)
 * it renders in place.
 *
 * `title` is the list screens' block (A-03, A-06, G-07): the page name large,
 * a one-line summary under it, and the actions on the right, inside the
 * content where the deck draws it.
 */
export function PageHeader(props: BarProps | TitleProps) {
  if (props.variant === "title") return <TitleBlock {...props} />;
  return <Bar {...props} />;
}

function Bar({ title, meta, actions, backTo }: BarProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const slot = useContext(PageBarSlot);

  const bar = (
    <header className="bg-background shrink-0 border-b">
      <div className="flex h-14 items-center gap-3 px-4">
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
  return slot === null ? bar : createPortal(bar, slot);
}

function TitleBlock({ title, subtitle, actions }: TitleProps) {
  return (
    <div className="flex items-end justify-between">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
        {subtitle === undefined ? null : (
          <p className="text-muted-foreground mt-0.5 text-sm">{subtitle}</p>
        )}
      </div>
      {actions === undefined ? null : (
        <div className="flex items-center gap-2">{actions}</div>
      )}
    </div>
  );
}
