import type { RefObject } from "react";
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

/**
 * GroupPreviewDialog shows the group as a student reads it. It stays mounted
 * while closed, so closing returns focus to `returnFocus`, the control that
 * opened it.
 */
export function GroupPreviewDialog({
  open,
  bundle,
  assets,
  returnFocus,
  onClose,
  onRefresh,
}: Readonly<{
  open: boolean;
  bundle: GroupBundle;
  assets: MediaAsset[];
  returnFocus: RefObject<HTMLElement | null>;
  onClose: () => void;
  onRefresh: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent
        className="max-h-[85svh] overflow-y-auto sm:max-w-4xl"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          returnFocus.current?.focus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{t("builder.previewAsStudent")}</DialogTitle>
          <DialogDescription>{t("builder.previewHint")}</DialogDescription>
        </DialogHeader>
        {bundle.questions.length ? (
          <PreviewBody bundle={bundle} assets={assets} onRefresh={onRefresh} />
        ) : (
          <p className="text-muted-foreground text-sm">{t("builder.previewEmpty")}</p>
        )}
      </DialogContent>
    </Dialog>
  );
}

function PreviewBody({
  bundle,
  assets,
  onRefresh,
}: Readonly<{ bundle: GroupBundle; assets: MediaAsset[]; onRefresh: () => void }>) {
  return (
    <StudentPreviewPane {...groupPreview(bundle, assets)} onRetryMedia={onRefresh} />
  );
}
