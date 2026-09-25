import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import type { MediaAsset } from "@/features/media/api";
import type { GroupBundle } from "../api";
import { groupPreview } from "../preview";

export function GroupPreviewDialog({
  bundle,
  assets,
  onClose,
  onRefresh,
}: Readonly<{
  bundle: GroupBundle;
  assets: MediaAsset[];
  onClose: () => void;
  onRefresh: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[85svh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{t("builder.previewAsStudent")}</DialogTitle>
          <DialogDescription>{t("builder.previewHint")}</DialogDescription>
        </DialogHeader>
        {bundle.questions.length ? (
          <StudentPreviewPane
            {...groupPreview(bundle, assets)}
            onRetryMedia={onRefresh}
          />
        ) : (
          <p className="text-muted-foreground text-sm">{t("builder.previewEmpty")}</p>
        )}
      </DialogContent>
    </Dialog>
  );
}
