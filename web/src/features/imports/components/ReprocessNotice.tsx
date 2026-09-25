import { useTranslation } from "react-i18next";
import type { WordImport } from "../api";
import { reprocessOutcome } from "../status";

/**
 * ReprocessNotice tells the teacher that the latest reprocess of an import
 * under review failed or was stopped, and that the current draft is unchanged.
 * It renders nothing otherwise.
 */
export function ReprocessNotice({
  value,
  className,
}: Readonly<{ value: WordImport; className?: string }>) {
  const { t } = useTranslation();
  const outcome = reprocessOutcome(value);
  if (outcome === null) return null;
  return <p className={className}>{t(`imports.reprocess.${outcome}`)}</p>;
}
