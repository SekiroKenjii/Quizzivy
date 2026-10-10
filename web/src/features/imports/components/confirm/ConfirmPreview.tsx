import { useId, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Monitor, Smartphone } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import { StudentPreview } from "@/features/tests";
import { cn } from "@/lib/utils";
import type { ImportDraftSection } from "../../api";
import { draftPreview } from "../../previewAdapter";

type Device = "desktop" | "phone";

/**
 * ConfirmPreview is "What students will see": every question still in the
 * import's saved draft, drawn by the student's paper with answers hidden, in
 * a computer-wide or a 390px phone frame, and a foot line counting them.
 */
export function ConfirmPreview({
  sections,
}: Readonly<{ sections: readonly ImportDraftSection[] }>) {
  const { t } = useTranslation();
  const titleId = useId();
  const [device, setDevice] = useState<Device>("desktop");
  const preview = useMemo(() => draftPreview(sections), [sections]);
  const phone = device === "phone";
  const shown = preview.questions.length;

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
      <div className="bg-sidebar max-h-160 overflow-y-auto p-5">
        <div
          data-preview-viewport={device}
          className={cn(
            "bg-bg mx-auto flex max-w-full min-w-0 flex-col gap-4.5 border p-5.5",
            phone ? "w-[390px] rounded-[24px]" : "w-full rounded-xl",
          )}
        >
          {shown === 0 ? (
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
          <p className="text-muted-fg text-meta text-center">
            {shown === preview.included
              ? t("imports.confirm.foot", { count: preview.included })
              : t("imports.confirm.footSome", { shown, count: preview.included })}
          </p>
        </div>
      </div>
    </section>
  );
}
