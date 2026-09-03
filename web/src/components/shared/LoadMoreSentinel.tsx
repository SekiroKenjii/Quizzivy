import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

/**
 * The last row of a lazy list: when it scrolls into view, the next page is
 * asked for. IntersectionObserver honours the clipping of every scrolling
 * ancestor, so the row counts as visible only once the reader has actually
 * reached the end of the box it lives in, not the end of the document.
 *
 * Says "đang tải thêm" while the page is on its way, so the end of the list
 * never looks like the end of the data.
 */
export function LoadMoreSentinel({
  as: Tag = "div",
  active,
  loading,
  onVisible,
}: {
  as?: "div" | "li";
  /** False once there is nothing more to load; the row then renders nothing. */
  active: boolean;
  loading: boolean;
  onVisible: () => void;
}) {
  const { t } = useTranslation();
  const ref = useRef<HTMLElement | null>(null);
  const latest = useRef(onVisible);
  useEffect(() => {
    latest.current = onVisible;
  });

  useEffect(() => {
    const el = ref.current;
    if (!active || el === null || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) latest.current();
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [active]);

  if (!active) return null;
  return (
    <Tag
      ref={ref as never}
      data-slot="load-more"
      aria-live="polite"
      className="text-muted-foreground px-2 py-1.5 text-center text-xs"
    >
      {loading ? t("common.loadingMore") : " "}
    </Tag>
  );
}
