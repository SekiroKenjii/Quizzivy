import { useId, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Monitor, Smartphone } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import {
  PANEL_FRAME,
  PaperPreview,
  PreviewFrameMessage,
  StudentPreviewPane,
  type PreviewDevice,
} from "@/features/tests";
import type { ImportDraftSection } from "../../api";
import { draftPreview } from "../../previewAdapter";

/**
 * ConfirmPreview is "What students will see": every question still in the
 * import's saved draft, on the take-test engine's preview with answers and
 * notes left out, in Test detail's computer or phone frame. What is chosen
 * there is kept only while the page is open.
 */
export function ConfirmPreview({
  title,
  sections,
}: Readonly<{ title: string; sections: readonly ImportDraftSection[] }>) {
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
            <PreviewFrameMessage>
              <p className="text-muted-fg text-base">
                {t("imports.confirm.previewEmpty")}
              </p>
            </PreviewFrameMessage>
          ) : (
            <PaperPreview
              title={title}
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
