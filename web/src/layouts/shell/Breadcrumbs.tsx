import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ChevronRight } from "lucide-react";
import type { PageCrumb } from "@/layouts/shell/crumbs";
import { cn } from "@/lib/utils";

/**
 * Breadcrumbs is the trail in the teacher shell's top bar. The last crumb is
 * the page on show, as text; each earlier crumb with an address is a link.
 * Every crumb ends in an ellipsis when the bar runs out of room, and below
 * 768px only the last one shows.
 */
export function Breadcrumbs({ trail }: Readonly<{ trail: readonly PageCrumb[] }>) {
  const { t } = useTranslation();
  const last = trail.length - 1;
  return (
    <nav aria-label={t("teacherShell.breadcrumb")} className="text-ui min-w-0">
      <ol className="flex min-w-0 items-center gap-1.5">
        {trail.map((crumb, position) => (
          <li
            key={`${crumb.to ?? ""}|${crumb.label}`}
            className={cn(
              "min-w-0 items-center gap-1.5",
              position === last ? "flex" : "hidden min-[768px]:flex",
            )}
          >
            {position > 0 && (
              <ChevronRight
                className="text-muted-fg hidden size-3.5 flex-none min-[768px]:block"
                aria-hidden="true"
              />
            )}
            <Crumb crumb={crumb} current={position === last} />
          </li>
        ))}
      </ol>
    </nav>
  );
}

function Crumb({ crumb, current }: Readonly<{ crumb: PageCrumb; current: boolean }>) {
  if (current) {
    return (
      <span aria-current="page" className="text-fg truncate leading-4.25 font-medium">
        {crumb.label}
      </span>
    );
  }
  if (crumb.to === undefined) {
    return <span className="text-muted-fg truncate leading-4.25">{crumb.label}</span>;
  }
  return (
    <Link to={crumb.to} className="text-muted-fg hover:text-fg truncate leading-4.25">
      {crumb.label}
    </Link>
  );
}
