import { useEffect, useEffectEvent, useRef } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { focusIsLost } from "@/lib/focusIsLost";
import { cn } from "@/lib/utils";

const BUTTON = "h-8.5 flex-none";
const PAUSED_SAVE = "aria-disabled:bg-primary/50 aria-disabled:hover:bg-primary/50";
const PAUSED_DISCARD = "aria-disabled:text-foreground/50";

/**
 * DirtyBar is the Teacher deck's unsaved-changes bar: it sticks 12px above
 * the bottom of the page while `dirty`, and renders nothing otherwise. It
 * reads "You have unsaved changes." until `error` is set, which takes the
 * sentence's place as an alert. Discard is a plain button. Save changes
 * submits the form the bar sits in, unless `onSave` is given, which makes it
 * a plain button too. While `saving` both stay focusable but answer
 * `aria-disabled` and ignore a press.
 *
 * When Save changes or Discard ends the unsaved changes, the bar goes and
 * would take keyboard focus with it, so focus moves to what `returnFocus`
 * returns, which each caller chooses. Focus the user has moved elsewhere in
 * the meantime is left alone.
 */
export function DirtyBar({
  dirty,
  saving = false,
  error = null,
  onDiscard,
  onSave,
  returnFocus,
}: Readonly<{
  dirty: boolean;
  saving?: boolean | undefined;
  error?: string | null | undefined;
  onDiscard: () => void;
  onSave?: (() => void) | undefined;
  returnFocus: () => HTMLElement | null;
}>) {
  const { t } = useTranslation();
  const pressed = useRef(false);
  const wasSaving = useRef(saving);
  const restoreFocus = useEffectEvent(() => returnFocus()?.focus());

  useEffect(() => {
    if (wasSaving.current && !saving && dirty) pressed.current = false;
    wasSaving.current = saving;
  }, [saving, dirty]);

  useEffect(() => {
    if (dirty || !pressed.current) return;
    pressed.current = false;
    if (focusIsLost()) restoreFocus();
  }, [dirty]);

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
          aria-disabled={saving || undefined}
          className={cn(
            BUTTON,
            PAUSED_DISCARD,
            "hover:bg-muted dark:hover:bg-muted px-3 in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm",
          )}
          onClick={() => {
            if (saving) return;
            pressed.current = true;
            onDiscard();
          }}
        >
          {t("controls.discard")}
        </Button>
        <Button
          type={onSave === undefined ? "submit" : "button"}
          aria-disabled={saving || undefined}
          className={cn(BUTTON, PAUSED_SAVE)}
          onClick={(event) => {
            if (saving) {
              event.preventDefault();
              return;
            }
            pressed.current = true;
            onSave?.();
          }}
        >
          {saving ? t("common.saving") : t("common.saveChanges")}
        </Button>
      </div>
    </div>
  );
}
