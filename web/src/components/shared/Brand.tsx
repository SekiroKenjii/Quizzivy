import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

/**
 * The brand kit on screen. Files are Thuong's, served from `public/brand/`
 * verbatim — never redrawn, recoloured, or re-lettered here.
 *
 * `<img>` rather than inline SVG for two reasons. Each variant is ~37 kB of
 * path data because the wordmark is Quicksand converted to outlines, and every
 * variant carries its own fixed colours, so there is nothing for `currentColor`
 * to do. Inlining would put all of that in the JS bundle to gain styling hooks
 * the kit forbids using.
 *
 * Width and height are always set from the kit's own viewBox, so the aspect
 * ratio cannot drift and the image reserves its box before it loads.
 */

// From docs/design/brand/svg/. The kit's B-05 sets these floors: below them a
// caller must switch to a smaller-lockup form rather than scaling this one
// down, because the concentric white gaps close up and the grey fold merges
// into the ring.
const ART = {
  lockup: {
    src: "/brand/quizzivy-logo-horizontal-color.svg",
    ratio: 885.5 / 205,
    floor: 120,
  },
  lockupOnDark: {
    src: "/brand/quizzivy-logo-horizontal-on-dark.svg",
    ratio: 885.5 / 205,
    floor: 120,
  },
  mark: { src: "/brand/quizzivy-mark-color.svg", ratio: 601.5 / 429.75, floor: 24 },
} as const;

type Art = keyof typeof ART;

function box(art: Art, height: number) {
  const { ratio, floor } = ART[art];
  const width = Math.round(height * ratio);
  // The floor is on width for the lockups and on height for the mark, which is
  // how the kit states them: a lockup runs out of room sideways, a mark
  // vertically.
  const measured = art === "mark" ? height : width;
  if (import.meta.env.DEV && measured < floor) {
    // A warning, never a throw. The first version of this threw, and a 2px
    // breach of a decorative recommendation took down a whole page behind the
    // error boundary -- a far worse outcome than a slightly small logo. The
    // floor is about legibility; nothing downstream depends on it.
    console.warn(
      `Brand: ${art} at height ${height} measures ${measured}px, under the kit's ${floor}px floor (B-05). ` +
        `Below a lockup's floor, switch to <BrandMark> rather than scaling the lockup down.`,
    );
  }
  return { width, height };
}

/**
 * The horizontal lockup — the mark and the wordmark together.
 *
 * `onDark` picks the variant drawn for a dark surface. It is not a theme
 * switch: the kit forbids putting `color` on a dark background at all, so this
 * follows the surface the caller is painting, not the user's theme.
 */
export function BrandLockup({
  height,
  onDark = false,
  className,
}: {
  height: number;
  onDark?: boolean;
  className?: string;
}) {
  const { t } = useTranslation();
  const art = onDark ? "lockupOnDark" : "lockup";
  return (
    <img
      src={ART[art].src}
      alt={t("app.name")}
      {...box(art, height)}
      className={cn("select-none", className)}
      draggable={false}
    />
  );
}

/**
 * The mark alone, beside the app's own wordmark as ordinary text.
 *
 * This is what B-04 prescribes wherever the lockup would fall under its floor —
 * the student top bar at 390px, where the lockup fits only 104px. The text is
 * Inter and deliberately so: the kit's lettering is Quicksand, and the rule is
 * that the two are never mixed. Type "Quizzivy" here and it is the product's
 * name in the product's typeface; reach for the logo and it is the file.
 */
export function BrandMark({
  height = 22,
  className,
  label = true,
}: {
  height?: number;
  className?: string;
  label?: boolean;
}) {
  const { t } = useTranslation();
  const mark = (
    <img
      src={ART.mark.src}
      // Decorative when the name follows it as text; the identifier otherwise.
      alt={label ? "" : t("app.name")}
      {...box("mark", height)}
      className="select-none"
      draggable={false}
    />
  );
  if (!label) return <span className={className}>{mark}</span>;
  return (
    <span className={cn("flex items-center gap-2", className)}>
      {mark}
      <span className="text-sm font-semibold tracking-tight">{t("app.name")}</span>
    </span>
  );
}
