import { useState } from "react";
import type { Editor } from "@tiptap/react";
import { ImagePlus, Music2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import type { MediaAsset, MediaKind } from "@/features/media/api";
import { materialTransaction } from "../materialTransaction";
import { MaterialAssetDialog } from "./MaterialAssetDialog";

export function MaterialTools({
  editor,
  onAsset,
}: Readonly<{ editor: Editor; onAsset: (asset: MediaAsset) => void }>) {
  const { t } = useTranslation();
  const [assetKind, setAssetKind] = useState<MediaKind | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [snapshot, setSnapshot] = useState(() => ({
    doc: editor.state.doc,
    selection: editor.state.selection.getBookmark(),
  }));

  function remember() {
    setError(null);
    setSnapshot({
      doc: editor.state.doc,
      selection: editor.state.selection.getBookmark(),
    });
  }
  function unchanged() {
    if (editor.isDestroyed || !editor.state.doc.eq(snapshot.doc)) {
      setError(t("groups.selectionChanged"));
      return false;
    }
    return true;
  }
  function insert(asset: MediaAsset, label: string) {
    if (!unchanged()) return;
    const transaction = materialTransaction(
      editor.state,
      snapshot.selection,
      asset,
      label,
    );
    if (!transaction) {
      setError(t("contentEditor.editBlocked"));
      return;
    }
    editor.view.dispatch(transaction);
    editor.view.focus();
    onAsset(asset);
    setAssetKind(null);
  }

  return (
    <>
      <div
        role="group"
        aria-label={t("groups.materialTools")}
        className="flex flex-wrap gap-1 border-b p-2"
      >
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            remember();
            setAssetKind("image");
          }}
        >
          <ImagePlus aria-hidden="true" />
          {t("groups.image")}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            remember();
            setAssetKind("audio");
          }}
        >
          <Music2 aria-hidden="true" />
          {t("groups.audio")}
        </Button>
      </div>
      {error ? (
        <p role="alert" className="px-3 py-2 text-sm">
          {error}
        </p>
      ) : null}
      {assetKind ? (
        <MaterialAssetDialog
          kind={assetKind}
          insertError={error}
          onClose={() => setAssetKind(null)}
          onInsert={insert}
        />
      ) : null}
    </>
  );
}
