import { useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageBarSlot } from "@/layouts/slots";

interface BarProps {
  variant?: "bar";
  title: string;
  /** Sits between the back arrow and the title: G-03's avatar. */
  leading?: ReactNode;
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

/** The one header every admin screen goes through, in the deck's two shapes. */
export function PageHeader(props: BarProps | TitleProps) {
  if (props.variant === "title") return <TitleBlock {...props} />;
  return <Bar {...props} />;
}

function Bar({ title, leading, meta, actions, backTo }: BarProps) {
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
        {leading}
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
