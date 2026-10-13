import { useCallback, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { EngineWidthContext } from "@/features/take-test/width";
import { cn } from "@/lib/utils";
import type { PreviewDevice, PreviewFrameSizes } from "./previewFrame";

const LG_PX = 1024;

/**
 * StudentPreviewPane is the frame every teacher preview of a paper draws:
 * a bordered screen on `--bg`, centred, at the size `sizes` gives `device`
 * (`PANEL_FRAME` or `WINDOW_FRAME`), holding a named region as tall as the
 * frame, where the engine's preview fills the screen and scrolls in its own
 * panes. The frame measures its own width and gives it to the engine's
 * readers through `EngineWidthContext`, and below 1024px it marks the region
 * `data-engine-narrow`, which the engine's `engine-lg:` sizes and its 44px
 * floor read, so a 390px phone frame draws the phone's layout and sizes on a
 * wide window; without a ResizeObserver they read the window's width.
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
  const [width, setWidth] = useState<number | null>(null);
  const measure = useCallback((frame: HTMLDivElement | null) => {
    if (frame === null || typeof ResizeObserver !== "function") return;
    const observer = new ResizeObserver(() => setWidth(frame.clientWidth));
    observer.observe(frame);
    return () => observer.disconnect();
  }, []);
  return (
    <div
      ref={measure}
      data-preview-viewport={device}
      className={cn(
        "bg-bg shadow-card mx-auto max-w-full min-w-0 overflow-hidden border",
        sizes[device],
      )}
    >
      <div
        role="region"
        aria-label={t("preview.frame")}
        data-engine-narrow={width !== null && width < LG_PX ? "" : undefined}
        className="relative flex h-full min-h-0 min-w-0 flex-col"
      >
        <EngineWidthContext value={width}>{children}</EngineWidthContext>
      </div>
    </div>
  );
}

/**
 * PreviewFrameMessage is what a frame holds while it has no paper to show:
 * the host's loading, error or empty state, padded and scrolling inside the
 * frame.
 */
export function PreviewFrameMessage({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <div className="relative flex min-h-0 flex-1 flex-col gap-4.5 overflow-y-auto overscroll-contain p-5.5">
      {children}
    </div>
  );
}
