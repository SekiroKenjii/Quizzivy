import { useTranslation } from "react-i18next";
import { FileText, Pencil, Settings2, ListTree, type LucideIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { ImportOrigin } from "../../api";

const ICON: Record<ImportOrigin, LucideIcon> = {
  source_explicit: FileText,
  inferred_structure: ListTree,
  defaulted: Settings2,
  teacher_entered: Pencil,
};

/** Provenance names where a value came from: the document, inferred structure, a default, or the teacher. */
export function Provenance({ origin }: Readonly<{ origin: ImportOrigin }>) {
  const { t } = useTranslation();
  const Icon = ICON[origin];
  return (
    <Badge variant={origin === "teacher_entered" ? "secondary" : "outline"}>
      <Icon aria-hidden="true" />
      {t(`imports.origin.${origin}`)}
    </Badge>
  );
}
