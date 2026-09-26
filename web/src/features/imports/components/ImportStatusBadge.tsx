import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import {
  Ban,
  CircleAlert,
  CircleCheck,
  Clock,
  Cog,
  FileUp,
  Hourglass,
  ListChecks,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { ImportStatus } from "../api";

type Variant = NonNullable<ComponentProps<typeof Badge>["variant"]>;

const LOOK: Record<ImportStatus, { icon: LucideIcon; variant: Variant }> = {
  awaiting_sources: { icon: FileUp, variant: "outline" },
  queued: { icon: Clock, variant: "outline" },
  processing: { icon: Cog, variant: "secondary" },
  needs_review: { icon: ListChecks, variant: "warning" },
  committing: { icon: Hourglass, variant: "secondary" },
  committed: { icon: CircleCheck, variant: "success" },
  failed: { icon: CircleAlert, variant: "danger" },
  cancelled: { icon: Ban, variant: "outline" },
};

/** ImportStatusBadge names an import's status in words beside an icon. */
export function ImportStatusBadge({ status }: Readonly<{ status: ImportStatus }>) {
  const { t } = useTranslation();
  const { icon: Icon, variant } = LOOK[status];
  return (
    <Badge variant={variant}>
      <Icon aria-hidden="true" />
      {t(`imports.status.${status}`)}
    </Badge>
  );
}
