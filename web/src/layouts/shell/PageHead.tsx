import type { ReactNode } from "react";
import { Link } from "react-router";
import { ArrowLeft } from "lucide-react";
import { useCrumbs } from "@/layouts/shell/crumbs";
import { cn } from "@/lib/utils";

/**
 * PageHead is the header of a page in the teacher shell: the one h1, with
 * `description` under it and `actions` at the end of the row, which wraps
 * beneath the title when the two do not fit. `back` and `status` make the
 * deck's detail form: a link back above the title and a pill beside it.
 * `children` follow the title in the same column, for a detail page's meta
 * line. `crumb` puts the title in the shell's breadcrumb trail and document
 * title as the page's own name; a page that calls useCrumbs itself leaves it
 * off. `className` restates a class of the row for a head the deck draws
 * differently.
 */
export function PageHead({
  title,
  description,
  actions,
  back,
  status,
  crumb = false,
  className,
  children,
}: Readonly<{
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  back?: { to: string; label: string };
  status?: ReactNode;
  crumb?: boolean;
  className?: string;
  children?: ReactNode;
}>) {
  useCrumbs(crumb ? [{ label: title }] : null);
  const detail = back !== undefined || status !== undefined;
  const heading = <h1 className="text-h1 min-w-0 break-words">{title}</h1>;
  return (
    <div
      data-slot="page-head"
      className={cn("flex flex-wrap items-end justify-between gap-3", className)}
    >
      <div className={cn("min-w-0", detail && "flex flex-[1_1_280px] flex-col gap-1")}>
        {back && (
          <Link
            to={back.to}
            className="text-muted-fg hover:text-fg inline-flex items-center gap-1.5 self-start text-sm leading-4 whitespace-nowrap"
          >
            <ArrowLeft className="size-3.5 flex-none" aria-hidden="true" />
            {back.label}
          </Link>
        )}
        {status === undefined ? (
          heading
        ) : (
          <div className="flex flex-wrap items-center gap-2.5">
            {heading}
            {status}
          </div>
        )}
        {description !== undefined && (
          <p className={cn("text-muted-fg text-base", !detail && "mt-0.5")}>
            {description}
          </p>
        )}
        {children}
      </div>
      {actions !== undefined && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  );
}
