/** PreviewDevice is the screen a preview frame stands for: a computer or a phone. */
export type PreviewDevice = "desktop" | "phone";

/**
 * PreviewFrameSizes gives, for each device, the classes that set a preview
 * frame's width, height and corner radius. The frame never grows past its
 * container's width.
 */
export type PreviewFrameSizes = Readonly<Record<PreviewDevice, string>>;

/**
 * PANEL_FRAME is the frame a page's preview card holds on Test detail and on
 * the import's "Preview and create": full width and 620px high with a 12px
 * radius, or a 390 × 720px phone with a 24px radius.
 */
export const PANEL_FRAME: PreviewFrameSizes = {
  desktop: "h-[620px] w-full rounded-xl",
  phone: "h-[720px] w-[390px] rounded-[24px]",
};

/**
 * WINDOW_FRAME is the frame of the builder's full-window preview: as tall as
 * its container, `min(1320px, 100%)` wide with a 12px radius, or 390px wide
 * with a 28px radius.
 */
export const WINDOW_FRAME: PreviewFrameSizes = {
  desktop: "h-full w-[min(1320px,100%)] rounded-xl",
  phone: "h-full w-[390px] rounded-[28px]",
};
