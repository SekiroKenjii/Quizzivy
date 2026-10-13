import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import type { PreviewDevice, PreviewFrameSizes } from "./previewFrame";

/**
 * StudentPreviewPane is the frame every teacher preview of a paper draws:
 * a bordered screen on `--bg`, centred, at the size `sizes` gives `device`
 * (`PANEL_FRAME` or `WINDOW_FRAME`), whose content scrolls inside it. The
 * scroller is a named region a keyboard can reach, so a paper with nothing
 * focusable still scrolls, and it positions what it holds, so a visually
 * hidden label inside cannot lengthen the page around it.
 */
export function StudentPreviewPane({
  device,
  sizes,
  children,
}: Readonly<{
  device: PreviewDevice;
  sizes: PreviewFrameSizes;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  return (
    <div
      data-preview-viewport={device}
      className={cn(
        "bg-bg shadow-card mx-auto max-w-full min-w-0 overflow-hidden border",
        sizes[device],
      )}
    >
      <div
        role="region"
        aria-label={t("preview.frame")}
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Scroll regions need keyboard access independently of their content.
        tabIndex={0}
        className="relative flex h-full min-w-0 flex-col gap-4.5 overflow-y-auto overscroll-contain p-5.5 -outline-offset-2"
      >
        {children}
      </div>
    </div>
  );
}
