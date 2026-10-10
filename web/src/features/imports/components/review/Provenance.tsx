import { useTranslation } from "react-i18next";
import { FileText, Pencil, Settings2, ScanSearch, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import type { ImportOrigin } from "../../api";

const ICON: Record<ImportOrigin, LucideIcon> = {
  source_explicit: FileText,
  inferred_structure: ScanSearch,
  defaulted: Settings2,
  teacher_entered: Pencil,
};

const TONE: Record<ImportOrigin, string> = {
  source_explicit: "bg-muted text-muted-fg",
  inferred_structure: "bg-info-soft text-info-ink",
  defaulted: "bg-warning-soft text-warning-ink",
  teacher_entered: "bg-brand-soft text-brand-ink",
};

/**
 * Provenance names where a value came from, as the deck's chip: the file,
 * inferred structure, a default, or the teacher ("Edited by you").
 */
export function Provenance({ origin }: Readonly<{ origin: ImportOrigin }>) {
  const { t } = useTranslation();
  const Icon = ICON[origin];
  return (
    <span
      data-origin={origin}
      className={cn(
        "text-2xs inline-flex h-5 items-center gap-1 rounded-[6px] px-1.75 font-medium whitespace-nowrap",
        TONE[origin],
      )}
    >
      <Icon aria-hidden="true" className="size-2.75 flex-none" />
      {t(`imports.origin.${origin}`)}
    </span>
  );
}
