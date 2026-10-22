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
import type { ProseDocument } from "../model";
import type { ProseField } from "../questionContent";
import { useProseMediaHost } from "./proseMedia";

/**
 * PastePreview makes clipboard conversion explicit before changing the
 * selected document, and says how many copied images were left out. In a
 * field that takes question media (`field`) it draws the media already there
 * and says an image can be pasted or dropped on its own.
 */
export function PastePreview({
  content,
  field,
  imagesLeftOut,
  onApply,
  onClose,
  onRestoreFocus,
}: Readonly<{
  content: ProseDocument | null;
  field?: ProseField | undefined;
  imagesLeftOut: number;
  onApply: () => void;
  onClose: () => void;
  onRestoreFocus: () => void;
}>) {
  const { t } = useTranslation();
  const host = useProseMediaHost();
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
              <ContentView
                document={content}
                field={field}
                renderMedia={host ? (node) => host.preview(node) : undefined}
              />
            </div>
          ) : (
            <p role="status">{t("contentEditor.pasteLoading")}</p>
          )}
          {imagesLeftOut > 0 && (
            <p className="text-muted-fg mt-2.5 flex items-start gap-2 text-[12.5px] leading-normal">
              <Info size={14} aria-hidden="true" className="mt-0.5 flex-none" />
              <span>
                {t("contentEditor.imagesLeftOut", { count: imagesLeftOut })}
                {field && host ? ` ${t("contentEditor.media.pasteOnItsOwn")}` : null}
              </span>
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
