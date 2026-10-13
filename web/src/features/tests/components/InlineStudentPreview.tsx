import { useState, type ComponentProps } from "react";
import { Monitor, Smartphone } from "lucide-react";
import { useTranslation } from "react-i18next";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { cn } from "@/lib/utils";
import { StudentPreview } from "./StudentPreview";

/**
 * InlineStudentPreview is the preview a dialog or a card holds in its own
 * flow: the learner content under a "Computer | Phone" toggle, at the
 * page's width or in a 320px phone column.
 */
export function InlineStudentPreview(
  props: Readonly<ComponentProps<typeof StudentPreview>>,
) {
  const { t } = useTranslation();
  const [mode, setMode] = useState("desktop");
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-muted-foreground text-xs">
          {mode === "phone" ? t("preview.phoneWidth") : t("preview.desktopWidth")}
        </p>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={mode}
          onValueChange={(value) => {
            if (value) setMode(value);
          }}
          aria-label={t("preview.displayMode")}
        >
          <ToggleGroupItem value="desktop">
            <Monitor aria-hidden="true" />
            {t("preview.desktop")}
          </ToggleGroupItem>
          <ToggleGroupItem value="phone">
            <Smartphone aria-hidden="true" />
            {t("preview.phone")}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
      <div
        data-preview-viewport={mode}
        className={cn(
          "mx-auto max-w-full min-w-0",
          mode === "phone" ? "w-[320px] rounded-xl border p-3" : "w-full",
        )}
      >
        <StudentPreview {...props} />
      </div>
    </div>
  );
}
