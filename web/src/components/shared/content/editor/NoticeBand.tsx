import { useTranslation } from "react-i18next";
import { TriangleAlert, X } from "lucide-react";

/** NoticeBand is the warning at the foot of the editor's box; it offers "Dismiss" when given onDismiss. */
export function NoticeBand({
  message,
  onDismiss,
}: Readonly<{ message: string; onDismiss?: () => void }>) {
  const { t } = useTranslation();
  return (
    <div
      role="alert"
      className="bg-warning-soft text-fg flex items-start gap-2.5 border-t py-2.5 pr-3 pl-3.5 text-[13px] leading-normal first:rounded-t-[9px] first:border-t-0 last:rounded-b-[9px]"
    >
      <TriangleAlert
        size={15}
        aria-hidden="true"
        className="text-warning-ink mt-0.5 flex-none"
      />
      <span className="min-w-0 flex-1 text-pretty">{message}</span>
      {onDismiss && (
        <button
          type="button"
          aria-label={t("contentEditor.dismiss")}
          title={t("contentEditor.dismiss")}
          onClick={onDismiss}
          className="text-muted-fg hover:bg-hover grid size-6 flex-none place-items-center rounded-[6px]"
        >
          <X size={14} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
