import { useTranslation } from "react-i18next";
import {
  CircleCheck,
  CircleDot,
  CircleSlash,
  CircleX,
  Loader,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { ImportStatus } from "../api";

type Pill = "processing" | "review" | "failed" | "draft" | "cancelled";

const PILLS: Record<Pill, { icon: LucideIcon; tone: string }> = {
  processing: { icon: Loader, tone: "bg-muted text-fg" },
  review: { icon: CircleDot, tone: "bg-info-soft text-info-ink" },
  failed: { icon: CircleX, tone: "bg-danger-soft text-danger-ink" },
  draft: { icon: CircleCheck, tone: "bg-success-soft text-success-ink" },
  cancelled: { icon: CircleSlash, tone: "bg-muted text-muted-fg" },
};

const PILL_OF: Record<ImportStatus, Pill> = {
  awaiting_sources: "processing",
  queued: "processing",
  processing: "processing",
  committing: "processing",
  needs_review: "review",
  committed: "draft",
  failed: "failed",
  cancelled: "cancelled",
};

/**
 * ImportStatusBadge names an import's status in words beside an icon, in the
 * look of the deck's five history pills: a status the server is still working
 * on, or one waiting for its files, takes the Processing pill's look.
 */
export function ImportStatusBadge({ status }: Readonly<{ status: ImportStatus }>) {
  const { t } = useTranslation();
  const { icon: Icon, tone } = PILLS[PILL_OF[status]];
  return (
    <Badge className={cn("h-6 rounded-full border-0 px-[9px]", tone)}>
      <Icon aria-hidden="true" />
      {t(`imports.status.${status}`)}
    </Badge>
  );
}
