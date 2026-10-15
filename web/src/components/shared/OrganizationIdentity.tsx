import { useState } from "react";
import { useTranslation } from "react-i18next";

import { logoFor, useOrganization } from "@/lib/organization";
import { useResolvedTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";

/**
 * OrganizationIdentity prints the centre on the front door: its logo, light
 * or dark by the theme on screen, and its name. `panel` is the brand panel's
 * line under the headline, `caption` the system pages' footnote. A logo that
 * is missing or will not load is left out, and with neither a logo nor a name
 * nothing is drawn. A logo made for the other theme sits on a tile of the
 * opposite ground, so it stays legible.
 */
export function OrganizationIdentity({
  variant,
}: Readonly<{ variant: "panel" | "caption" }>) {
  const { t } = useTranslation();
  const organization = useOrganization();
  const theme = useResolvedTheme();
  const [broken, setBroken] = useState<string | null>(null);
  const logo = logoFor(organization, theme);
  const src = logo !== null && logo.url !== broken ? logo.url : null;
  const flipped = src !== null && logo?.flipped === true;
  const { name } = organization;
  if (src === null && name === null) return null;

  const panel = variant === "panel";
  const image = src && (
    <img
      src={src}
      alt={name ? "" : t("app.name")}
      draggable={false}
      onError={() => setBroken(src)}
      className={panel ? "size-8 object-contain" : "size-5 object-contain"}
    />
  );
  if (panel) {
    return (
      <div className="flex items-center gap-3">
        {image && (
          <span
            className={cn(
              "grid size-11 shrink-0 place-items-center rounded-xl border",
              flipped ? "bg-fg" : "bg-card",
            )}
          >
            {image}
          </span>
        )}
        {name && (
          <p className="text-muted-fg text-md min-w-0 leading-relaxed break-words">
            {name}
          </p>
        )}
      </div>
    );
  }
  return (
    <span className="flex items-center gap-2">
      {image && (
        <span
          className={cn(
            "grid size-6 shrink-0 place-items-center rounded-md",
            flipped && "bg-fg",
          )}
        >
          {image}
        </span>
      )}
      {name && (
        <span title={name} className="min-w-0 truncate">
          {name}
        </span>
      )}
    </span>
  );
}
