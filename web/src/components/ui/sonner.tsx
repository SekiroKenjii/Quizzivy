import { CircleAlert, CircleCheck, Info, TriangleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Toaster as Sonner, toast } from "sonner";

/**
 * Toaster is the deck's toast: a card at the bottom right, one at a time, with
 * an icon in the tone's colour. Plain `toast()` calls keep a neutral card; the
 * tones come from `notify` in lib/toast.ts.
 */
function Toaster() {
  const { t } = useTranslation();
  return (
    <Sonner
      position="bottom-right"
      duration={4000}
      gap={8}
      offset={16}
      visibleToasts={1}
      containerAriaLabel={t("toast.region")}
      icons={{
        success: <CircleCheck aria-hidden="true" className="text-success size-4" />,
        info: <Info aria-hidden="true" className="text-info-ink size-4" />,
        warning: (
          <TriangleAlert aria-hidden="true" className="text-warning-ink size-4" />
        ),
        error: <CircleAlert aria-hidden="true" className="text-danger-ink size-4" />,
      }}
      toastOptions={{
        unstyled: true,
        closeButtonAriaLabel: t("toast.close"),
        classNames: {
          toast:
            "bg-card text-fg shadow-float flex w-auto max-w-[calc(100vw-2rem)] items-center gap-2.5 rounded-[10px] border px-3.5 py-3 text-ui",
          icon: "flex size-4 shrink-0 items-center justify-center",
          title: "min-w-0 flex-1 font-normal",
          actionButton:
            "ml-auto shrink-0 text-sm font-medium underline-offset-4 hover:underline",
          closeButton:
            "text-muted-fg hover:bg-hover order-last ml-1 grid size-6 shrink-0 place-items-center rounded-md",
        },
      }}
    />
  );
}

export { Toaster, toast };
