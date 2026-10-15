import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { RoleSummary } from "@/features/roles/api";
import { RoleCard } from "@/features/roles/components/RoleCard";
import { useContentBand } from "@/layouts/shell/contentWidth";
import { focusIsLost } from "@/lib/focusIsLost";
import { cn } from "@/lib/utils";

const BREAKPOINTS = [640, 1000] as const;

const SLIDE = [
  "basis-[86%]",
  "basis-[calc((100%-0.75rem)/2)]",
  "basis-[calc((100%-1.5rem)/3)]",
] as const;

const ARROW =
  "border bg-card text-fg shadow-float absolute top-1/2 z-2 grid size-10 -translate-y-1/2 place-items-center rounded-full transition-opacity hover:bg-muted disabled:pointer-events-none disabled:opacity-0 motion-reduce:transition-none";

type Arrow = "previous" | "next";

/**
 * RoleCarousel lays the role cards in a scroll-snap track: three to a view
 * when the page's content is at least 1000px wide, two from 640 and else one
 * at 86%, so the next card shows. Round arrows fade at the ends and leave the
 * tab order, and one dot stands for each place the track can rest, the
 * current one 22px wide. `reveal` scrolls the card of that role id into view, once for each id.
 * `action` draws the control at the end of each card's first row.
 */
export function RoleCarousel({
  roles,
  reveal = null,
  action,
  onEditPermissions,
}: Readonly<{
  roles: readonly RoleSummary[];
  reveal?: string | null;
  action?: ((role: RoleSummary) => ReactNode) | undefined;
  onEditPermissions: (role: RoleSummary) => void;
}>) {
  const { t } = useTranslation();
  const per = useContentBand(BREAKPOINTS) + 1;
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");
  const track = useRef<HTMLDivElement>(null);
  const previous = useRef<HTMLButtonElement>(null);
  const next = useRef<HTMLButtonElement>(null);
  const lastArrow = useRef<Arrow | null>(null);
  const revealed = useRef<string | null>(null);
  const [rested, setRested] = useState(0);
  const pages = Math.max(1, roles.length - per + 1);
  const at = Math.min(rested, pages - 1);
  const behavior = reduced ? "auto" : "smooth";

  const goTo = useCallback(
    (page: number) => {
      const element = track.current;
      const card = element?.children.item(page);
      if (element && card instanceof HTMLElement)
        element.scrollTo({ left: card.offsetLeft, behavior });
    },
    [behavior],
  );

  useEffect(() => {
    if (reveal === null || revealed.current === reveal) return;
    const index = roles.findIndex((role) => role.id === reveal);
    if (index < 0) return;
    revealed.current = reveal;
    goTo(Math.min(index, pages - 1));
  }, [reveal, roles, pages, goTo]);

  useEffect(() => {
    const arrow = lastArrow.current;
    if (arrow === null) return;
    const pressed = (arrow === "next" ? next : previous).current;
    if (pressed === null || !pressed.disabled) return;
    lastArrow.current = null;
    if (!focusIsLost() && document.activeElement !== pressed) return;
    (arrow === "next" ? previous : next).current?.focus();
  }, [at]);

  function settle() {
    const element = track.current;
    if (element === null) return;
    let nearest = 0;
    let best = Number.POSITIVE_INFINITY;
    Array.from(element.children).forEach((card, index) => {
      if (!(card instanceof HTMLElement)) return;
      const distance = Math.abs(card.offsetLeft - element.scrollLeft);
      if (distance < best) {
        best = distance;
        nearest = index;
      }
    });
    const end = element.scrollLeft + element.clientWidth >= element.scrollWidth - 4;
    setRested(end ? pages - 1 : Math.min(nearest, pages - 1));
  }

  function step(arrow: Arrow, to: number) {
    lastArrow.current = arrow;
    goTo(to);
  }

  function forgetArrow(related: EventTarget | null) {
    if (related !== null) lastArrow.current = null;
  }

  return (
    <section
      aria-label={t("roles.carousel")}
      aria-roledescription={t("roles.carouselKind")}
      className="flex flex-col gap-3"
    >
      <div className="relative">
        <div
          ref={track}
          onScroll={settle}
          className="relative flex snap-x snap-mandatory [scrollbar-width:none] gap-3 overflow-x-auto overscroll-x-contain rounded-[1rem] motion-safe:scroll-smooth [&::-webkit-scrollbar]:hidden"
        >
          {roles.map((role) => (
            <RoleCard
              key={role.id}
              role={role}
              className={SLIDE[per - 1]}
              action={action?.(role)}
              onEditPermissions={onEditPermissions}
            />
          ))}
        </div>
        <button
          ref={previous}
          type="button"
          disabled={at === 0}
          aria-label={t("roles.previous")}
          className={cn(ARROW, "-left-1.5")}
          onClick={() => step("previous", Math.max(0, at - 1))}
          onBlur={(event) => forgetArrow(event.relatedTarget)}
        >
          <ChevronLeft aria-hidden="true" className="size-4.5" />
        </button>
        <button
          ref={next}
          type="button"
          disabled={at >= pages - 1}
          aria-label={t("roles.next")}
          className={cn(ARROW, "-right-1.5")}
          onClick={() => step("next", Math.min(pages - 1, at + 1))}
          onBlur={(event) => forgetArrow(event.relatedTarget)}
        >
          <ChevronRight aria-hidden="true" className="size-4.5" />
        </button>
      </div>
      {pages > 1 && (
        <div className="flex justify-center">
          {Array.from({ length: pages }, (_, page) => (
            <button
              key={page}
              type="button"
              aria-label={t("roles.slide", { n: page + 1 })}
              aria-current={page === at || undefined}
              className="grid h-6 place-items-center px-[3px]"
              onClick={() => goTo(page)}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "h-1.5 rounded-md transition-[width] duration-200 motion-reduce:transition-none",
                  page === at ? "bg-fg w-5.5" : "bg-border w-1.5",
                )}
              />
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
