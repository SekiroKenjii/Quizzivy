import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const BUTTON = "h-8.5 flex-none";

/**
 * DirtyBar is the Teacher deck's unsaved-changes bar: it sticks 12px above
 * the bottom of the page while `dirty`, and renders nothing otherwise. It
 * reads "You have unsaved changes." until `error` is set, which takes the
 * sentence's place as an alert. Discard is a plain button. Save changes
 * submits the form the bar sits in, unless `onSave` is given, which makes it
 * a plain button too. Both are off while `saving`.
 */
export function DirtyBar({
  dirty,
  saving = false,
  error = null,
  onDiscard,
  onSave,
}: Readonly<{
  dirty: boolean;
  saving?: boolean | undefined;
  error?: string | null | undefined;
  onDiscard: () => void;
  onSave?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  if (!dirty) return null;
  return (
    <div
      data-slot="dirty-bar"
      className="bg-card shadow-float sticky bottom-3 flex flex-wrap items-center gap-2.5 rounded-xl border py-2.5 pr-3 pl-4"
    >
      {error === null || error === "" ? (
        <p className="text-muted-fg min-w-40 flex-1 text-sm">{t("controls.unsaved")}</p>
      ) : (
        <p role="alert" className="text-danger-ink min-w-40 flex-1 text-sm">
          {error}
        </p>
      )}
      <div className="ml-auto flex items-center gap-2.5">
        <Button
          type="button"
          variant="ghost"
          disabled={saving}
          className={cn(
            BUTTON,
            "hover:bg-muted dark:hover:bg-muted px-3 in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm",
          )}
          onClick={() => onDiscard()}
        >
          {t("controls.discard")}
        </Button>
        <Button
          type={onSave === undefined ? "submit" : "button"}
          disabled={saving}
          className={BUTTON}
          onClick={onSave === undefined ? undefined : () => onSave()}
        >
          {saving ? t("common.saving") : t("common.saveChanges")}
        </Button>
      </div>
    </div>
  );
}
