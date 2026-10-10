import { useLayoutEffect, useRef, useState } from "react";

import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";

/**
 * MarqueeText reads text once, exposes cut titles in full, and scrolls
 * overflow with hover/focus pauses or a reduced-motion ellipsis. Its in-flow
 * content is one copy of the text whether it scrolls or not, so a host that
 * sizes it by its content gives it the same width in both states and the
 * overflow check cannot flip it back and forth.
 */
export function MarqueeText({
  text,
  minSeconds = 8,
  gapPx,
  maskPx,
  className,
}: Readonly<{
  text: string;
  minSeconds?: number;
  gapPx?: number;
  maskPx?: number;
  className?: string;
}>) {
  const box = useRef<HTMLSpanElement>(null);
  const measure = useRef<HTMLSpanElement>(null);
  const [overflows, setOverflows] = useState(false);
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");

  const moving = overflows && !reduced;

  useLayoutEffect(() => {
    const outer = box.current;
    const inner = measure.current;
    if (!outer || !inner) return;
    const check = () =>
      setOverflows(inner.getBoundingClientRect().width > outer.clientWidth + 1);
    check();
    if (typeof ResizeObserver !== "function") return;
    const observer = new ResizeObserver(check);
    observer.observe(outer);
    observer.observe(inner);
    return () => observer.disconnect();
  }, [text, moving]);

  const seconds = Math.max(minSeconds, Math.round(text.length / 4));

  return (
    <span
      ref={box}
      title={overflows ? text : undefined}
      style={
        moving && maskPx !== undefined
          ? {
              maskImage: `linear-gradient(90deg, transparent, var(--foreground) ${maskPx}px, var(--foreground) calc(100% - ${maskPx}px), transparent)`,
            }
          : undefined
      }
      className={cn(
        "qz-marquee relative block min-w-0 overflow-hidden whitespace-nowrap",
        moving ? "qz-marquee-masked" : "text-ellipsis",
        className,
      )}
    >
      <span
        ref={measure}
        data-slot="marquee-measure"
        aria-hidden="true"
        className="pointer-events-none invisible absolute top-0 left-0 w-max max-w-none whitespace-nowrap"
      >
        {text}
      </span>
      {moving ? (
        <>
          <span data-slot="marquee-sizer" aria-hidden="true" className="invisible">
            {text}
          </span>
          <span
            className="qz-marquee-track absolute top-0 left-0 inline-flex w-max"
            style={{ animationDuration: `${seconds}s` }}
          >
            <span>{text}</span>
            <span
              aria-hidden="true"
              className={gapPx === undefined ? "pl-8" : undefined}
              style={gapPx === undefined ? undefined : { paddingLeft: gapPx }}
            >
              {text}
            </span>
          </span>
        </>
      ) : (
        <span>{text}</span>
      )}
    </span>
  );
}
