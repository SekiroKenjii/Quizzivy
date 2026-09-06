import type { ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";

type Variant = NonNullable<ComponentProps<typeof Badge>["variant"]>;

/** F-07: every state in the domain has exactly one badge, and never two colours. */
export const STATUS_BADGES = {
  test: { draft: "secondary", published: "success", archived: "outline" },
  assignment: {
    draft: "secondary",
    scheduled: "outline",
    open: "success",
    closed: "secondary",
  },
  attempt: {
    not_started: "outline",
    in_progress: "primary",
    submitted: "secondary",
    timed_out: "warning",
    graded: "success",
    voided: "danger",
  },
  attention: { flagged: "warning", pendingManual: "outline", audio: "outline" },
} as const satisfies Record<string, Record<string, Variant>>;

export type StatusKind = keyof typeof STATUS_BADGES;
export type StatusOf<K extends StatusKind> = keyof (typeof STATUS_BADGES)[K] & string;

export function StatusBadge<K extends StatusKind>({
  kind,
  status,
  ...props
}: { kind: K; status: StatusOf<K> } & Omit<
  ComponentProps<typeof Badge>,
  "variant" | "children"
>) {
  const { t } = useTranslation();
  const variant = (STATUS_BADGES[kind] as Record<string, Variant>)[status] as Variant;
  return (
    <Badge variant={variant} {...props}>
      {t(`status.${kind}.${status}`)}
    </Badge>
  );
}
