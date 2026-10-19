import { createElement } from "react";
import { toast } from "sonner";

type Options = Parameters<typeof toast.success>[1];

/**
 * notify shows a toast in one of the deck's four tones. Success and info leave
 * after 2.4 s, as the deck's do; warnings and errors stay 6 s with a close
 * button, and an error is announced at once rather than when the reader is
 * idle.
 */
export const notify = {
  success: (message: string, options?: Options) =>
    toast.success(message, { duration: 2400, ...options }),
  info: (message: string, options?: Options) =>
    toast.info(message, { duration: 2400, ...options }),
  warning: (message: string, options?: Options) =>
    toast.warning(message, { duration: 6000, closeButton: true, ...options }),
  error: (message: string, options?: Options) =>
    toast.error(createElement("span", { role: "alert" }, message), {
      duration: 6000,
      closeButton: true,
      ...options,
    }),
  dismiss: (id: string | number) => toast.dismiss(id),
};

/**
 * leaveToast takes focus out of the toaster when it is inside, for an action
 * that opens something else. Sonner returns focus to where it was before the
 * toasts took it, and does so when focus leaves the toaster or, if it has not
 * left yet, when the toast is removed 200ms later, which would take focus
 * from whatever the action opened.
 */
export function leaveToast() {
  const active = document.activeElement;
  if (active instanceof HTMLElement && active.closest("[data-sonner-toaster]"))
    active.blur();
}
