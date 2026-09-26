import { useLayoutEffect, useRef, useState } from "react";

import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";

/**
 * MarqueeText is one line of text that scrolls sideways only when it does not
 * fit, the deck's answer to long titles. It pauses under the pointer and while
 * it or its control has keyboard focus, and under reduced motion it truncates
 * with an ellipsis instead. Assistive technology reads the text once; the full
 * text is also the element's title whenever it is cut.
 */
export function MarqueeText({
  text,
  minSeconds = 8,
  className,
}: Readonly<{ text: string; minSeconds?: number; className?: string }>) {
  const box = useRef<HTMLSpanElement>(null);
  const measure = useRef<HTMLSpanElement>(null);
  const [overflows, setOverflows] = useState(false);
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");

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
  }, [text]);

  const moving = overflows && !reduced;
  const seconds = Math.max(minSeconds, Math.round(text.length / 4));

  return (
    <span
      ref={box}
      title={overflows ? text : undefined}
      className={cn(
        "qz-marquee block min-w-0 overflow-hidden whitespace-nowrap",
        moving ? "qz-marquee-masked" : "text-ellipsis",
        className,
      )}
    >
      {moving ? (
        <span
          className="qz-marquee-track inline-flex"
          style={{ animationDuration: `${seconds}s` }}
        >
          <span ref={measure}>{text}</span>
          <span aria-hidden="true" className="pl-8">
            {text}
          </span>
        </span>
      ) : (
        <span ref={measure}>{text}</span>
      )}
    </span>
  );
}
