import { useEffect, useEffectEvent, useId, useRef } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { focusIsLost } from "@/lib/focusIsLost";
import { cn } from "@/lib/utils";

const BUTTON = "h-8.5 flex-none";
const PAUSED_SAVE = "aria-disabled:bg-primary/50 aria-disabled:hover:bg-primary/50";
const PAUSED_DISCARD = "aria-disabled:text-foreground/50";
const PAUSED_OUTLINE =
  "aria-disabled:text-foreground/50 aria-disabled:border-border/50";

/**
 * DirtyBar is the Teacher deck's unsaved-changes bar: it sticks 12px above
 * the bottom of the page while `dirty`, and renders nothing otherwise. It
 * reads "You have unsaved changes." until `error` is set, which takes the
 * sentence's place as an alert, with `errorAction`, when given, as a button
 * beside Discard. Discard is a plain button. Save changes submits the form
 * the bar sits in, unless `onSave` is given, which makes it a plain button
 * too. While `saving` both stay focusable but answer `aria-disabled` and
 * ignore a press. While `blocked` names a reason, Save changes alone does the
 * same, and the reason reads in place of the sentence and describes the
 * button.
 *
 * When Save changes or Discard ends the unsaved changes, the bar goes and
 * would take keyboard focus with it, so focus moves to what `returnFocus`
 * returns, which each caller chooses. Focus the user has moved elsewhere in
 * the meantime is left alone.
 */
export function DirtyBar({
  dirty,
  saving = false,
  blocked = null,
  error = null,
  errorAction,
  onDiscard,
  onSave,
  returnFocus,
}: Readonly<{
  dirty: boolean;
  saving?: boolean | undefined;
  blocked?: string | null | undefined;
  error?: string | null | undefined;
  errorAction?: Readonly<{ label: string; onAction: () => void }> | undefined;
  onDiscard: () => void;
  onSave?: (() => void) | undefined;
  returnFocus: () => HTMLElement | null;
}>) {
  const { t } = useTranslation();
  const reasonId = useId();
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

  const failed = error !== null && error !== "";
  const reason = blocked === null || blocked === "" ? null : blocked;
  const paused = saving || reason !== null;

  return (
    <div
      data-slot="dirty-bar"
      className="bg-card shadow-float sticky bottom-3 flex flex-wrap items-center gap-2.5 rounded-xl border py-2.5 pr-3 pl-4"
    >
      {failed ? (
        <p role="alert" className="text-danger-ink min-w-40 flex-1 text-sm">
          {error}
        </p>
      ) : (
        <p
          id={reasonId}
          className={cn(
            "min-w-40 flex-1 text-sm",
            reason === null ? "text-muted-fg" : "text-danger-ink",
          )}
        >
          {reason ?? t("controls.unsaved")}
        </p>
      )}
      <div className="ml-auto flex items-center gap-2.5">
        {failed && errorAction !== undefined ? (
          <Button
            type="button"
            variant="outline"
            aria-disabled={saving || undefined}
            className={cn(BUTTON, PAUSED_OUTLINE)}
            onClick={() => {
              if (!saving) errorAction.onAction();
            }}
          >
            {errorAction.label}
          </Button>
        ) : null}
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
          aria-disabled={paused || undefined}
          aria-describedby={reason !== null && !failed ? reasonId : undefined}
          className={cn(BUTTON, PAUSED_SAVE)}
          onClick={(event) => {
            if (paused) {
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
