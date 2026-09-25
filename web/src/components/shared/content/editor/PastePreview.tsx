import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
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

/** PastePreview makes clipboard conversion explicit before changing the selected document. */
export function PastePreview({
  content,
  failed,
  onApply,
  onClose,
  onRestoreFocus,
}: Readonly<{
  content: SemanticContent | null;
  failed: boolean;
  onApply: () => void;
  onClose: () => void;
  onRestoreFocus: () => void;
}>) {
  const { t } = useTranslation();
  let preview = <p role="status">{t("contentEditor.pasteLoading")}</p>;
  if (content)
    preview = (
      <div className="min-h-0 overflow-auto rounded-md border p-4">
        <ContentView document={content} />
      </div>
    );
  if (failed)
    preview = (
      <Alert>
        <AlertDescription>{t("contentEditor.pasteBlocked")}</AlertDescription>
      </Alert>
    );
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        className="flex max-h-[85dvh] min-w-0 flex-col sm:max-w-2xl"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          onRestoreFocus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{t("contentEditor.pasteTitle")}</DialogTitle>
          <DialogDescription>{t("contentEditor.pasteDescription")}</DialogDescription>
        </DialogHeader>
        {preview}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button type="button" onClick={onApply} disabled={!content || failed}>
            {t("contentEditor.pasteApply")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
