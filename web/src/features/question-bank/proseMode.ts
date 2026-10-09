import { useState } from "react";

/** ProseMode is the form a prompt or an explanation is stored in: rich content, or a Markdown string. */
export type ProseMode = "rich" | "markdown";

/** ProseStep is what the field shows besides its body: nothing, the "Switch to Markdown" question, or the conversion to rich text. */
export type ProseStep = "edit" | "leaving" | "converting";

/**
 * useProseMode holds a field's mode and the confirmation it is in. `ask`
 * opens the confirmation for the other mode and never changes the mode;
 * `finish` records the mode a confirmed switch stored.
 */
export function useProseMode(initial: () => ProseMode) {
  const [mode, setMode] = useState(initial);
  const [step, setStep] = useState<ProseStep>("edit");
  return {
    mode,
    step,
    ask: (next: ProseMode) => {
      if (next !== mode) setStep(next === "markdown" ? "leaving" : "converting");
    },
    cancel: () => setStep("edit"),
    finish: (next: ProseMode) => {
      setMode(next);
      setStep("edit");
    },
  };
}

/** focusOpener returns focus to the header's unselected mode, the one that opened a confirmation. */
export function focusOpener(header: HTMLElement | null) {
  header?.querySelector<HTMLButtonElement>('button[aria-pressed="false"]')?.focus();
}
