import { useTranslation } from "react-i18next";
import { Info } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ContentView } from "../ContentView";
import type { SemanticContent } from "../model";

/** PastePreview makes clipboard conversion explicit before changing the selected document, and says how many copied images were left out. */
export function PastePreview({
  content,
  imagesLeftOut,
  onApply,
  onClose,
  onRestoreFocus,
}: Readonly<{
  content: SemanticContent | null;
  imagesLeftOut: number;
  onApply: () => void;
  onClose: () => void;
  onRestoreFocus: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        className="flex max-h-[85dvh] min-w-0 flex-col sm:max-w-[640px]"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          onRestoreFocus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{t("contentEditor.pasteTitle")}</DialogTitle>
          <DialogDescription>{t("contentEditor.pasteDescription")}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto">
          {content ? (
            <div className="content-editor rounded-[9px] border px-4 py-3.5 text-[14.5px] leading-[1.6]">
              <ContentView document={content} />
            </div>
          ) : (
            <p role="status">{t("contentEditor.pasteLoading")}</p>
          )}
          {imagesLeftOut > 0 && (
            <p className="text-muted-fg mt-2.5 flex items-start gap-2 text-[12.5px] leading-normal">
              <Info size={14} aria-hidden="true" className="mt-0.5 flex-none" />
              {t("contentEditor.imagesLeftOut", { count: imagesLeftOut })}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button type="button" onClick={onApply} disabled={!content}>
            {t("contentEditor.pasteApply")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
