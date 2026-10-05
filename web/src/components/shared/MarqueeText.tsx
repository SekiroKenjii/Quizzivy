import { useLayoutEffect, useRef, useState } from "react";

import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";

/** MarqueeText reads text once, exposes cut titles in full, and scrolls overflow with hover/focus pauses or a reduced-motion ellipsis. */
export function MarqueeText({
  text,
  minSeconds = 8,
  className,
}: Readonly<{ text: string; minSeconds?: number; className?: string }>) {
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
        <span
          className="qz-marquee-track inline-flex"
          style={{ animationDuration: `${seconds}s` }}
        >
          <span>{text}</span>
          <span aria-hidden="true" className="pl-8">
            {text}
          </span>
        </span>
      ) : (
        <span>{text}</span>
      )}
    </span>
  );
}
