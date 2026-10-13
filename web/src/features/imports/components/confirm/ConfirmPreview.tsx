import { useId, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Monitor, Smartphone } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import {
  PANEL_FRAME,
  StudentPreview,
  StudentPreviewPane,
  type PreviewDevice,
} from "@/features/tests";
import type { ImportDraftSection } from "../../api";
import { draftPreview } from "../../previewAdapter";

/**
 * ConfirmPreview is "What students will see": every question still in the
 * import's saved draft, drawn by the student's paper with answers and notes
 * left out, in Test detail's computer or phone frame.
 */
export function ConfirmPreview({
  sections,
}: Readonly<{ sections: readonly ImportDraftSection[] }>) {
  const { t } = useTranslation();
  const titleId = useId();
  const [device, setDevice] = useState<PreviewDevice>("desktop");
  const preview = useMemo(() => draftPreview(sections), [sections]);

  return (
    <section
      aria-labelledby={titleId}
      className="bg-card shadow-card min-w-0 overflow-hidden rounded-xl border"
    >
      <div className="flex flex-wrap items-center justify-between gap-2.5 border-b px-4 py-3">
        <h2 id={titleId} className="text-base font-semibold">
          {t("imports.confirm.previewTitle")}
        </h2>
        <Segmented
          size="sm"
          label={t("imports.confirm.device")}
          value={device}
          onChange={(value) => setDevice(value === "phone" ? "phone" : "desktop")}
          options={[
            { value: "desktop", label: t("imports.confirm.computer"), icon: Monitor },
            { value: "phone", label: t("imports.confirm.phone"), icon: Smartphone },
          ]}
        />
      </div>
      <div className="bg-sidebar p-4">
        <StudentPreviewPane device={device} sizes={PANEL_FRAME}>
          {preview.questions.length === 0 ? (
            <p className="text-muted-fg text-base">
              {t("imports.confirm.previewEmpty")}
            </p>
          ) : (
            <StudentPreview
              questions={preview.questions}
              sections={preview.sections}
              groups={preview.groups}
            />
          )}
        </StudentPreviewPane>
      </div>
    </section>
  );
}
