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
};
