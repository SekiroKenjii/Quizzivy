import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import { useResolvedTheme } from "@/lib/theme";

/**
 * The brand kit on screen. Files are Thuong's, served from `public/brand/`
 * verbatim — never redrawn, recoloured, or re-lettered here.
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
  markOnDark: {
    src: "/brand/quizzivy-mark-on-dark.svg",
    ratio: 601.5 / 429.75,
    floor: 24,
  },
} as const;

type Art = keyof typeof ART;

function box(art: Art, height: number) {
  const { ratio, floor } = ART[art];
  const width = Math.round(height * ratio);
  const measured = art === "mark" || art === "markOnDark" ? height : width;
  if (import.meta.env.DEV && measured < floor) {
    // A warning, never a throw.
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
 * `onDark` picks the variant drawn for a surface that is dark in both themes.
 * `theme="auto"` follows the theme on screen instead, for surfaces painted
 * with the page background. The kit forbids `color` on a dark ground.
 */
export function BrandLockup({
  height,
  onDark = false,
  theme,
  className,
}: Readonly<{
  height: number;
  onDark?: boolean;
  theme?: "auto";
  className?: string;
}>) {
  const { t } = useTranslation();
  const resolved = useResolvedTheme();
  const dark = onDark || (theme === "auto" && resolved === "dark");
  const art = dark ? "lockupOnDark" : "lockup";
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

/** The mark alone, beside the app's own wordmark as ordinary text. */
export function BrandMark({
  height = 22,
  className,
  label = true,
  theme,
}: Readonly<{
  height?: number;
  className?: string;
  label?: boolean;
  theme?: "auto";
}>) {
  const { t } = useTranslation();
  const resolved = useResolvedTheme();
  const art = theme === "auto" && resolved === "dark" ? "markOnDark" : "mark";
  const mark = (
    <img
      src={ART[art].src}
      // Decorative when the name follows it as text; the identifier otherwise.
      alt={label ? "" : t("app.name")}
      {...box(art, height)}
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
