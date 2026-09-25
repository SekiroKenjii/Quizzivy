import { useState } from "react";
import { useEditorState, type Editor } from "@tiptap/react";
import { ImagePlus, Link, Music2, Unlink } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { safeContentURL } from "@/components/shared/content/validation";
import type { MediaAsset, MediaKind } from "@/features/media/api";
import { fromEditorDoc } from "@/components/shared/content/editor/adapter";
import { materialTransaction } from "../materialTransaction";
import { MaterialAssetDialog } from "./MaterialAssetDialog";

export function MaterialTools({
  editor,
  onAsset,
}: Readonly<{ editor: Editor; onAsset: (asset: MediaAsset) => void }>) {
  const { t } = useTranslation();
  const [assetKind, setAssetKind] = useState<MediaKind | null>(null);
  const [linking, setLinking] = useState(false);
  const [url, setURL] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [snapshot, setSnapshot] = useState(() => ({
    doc: editor.state.doc,
    from: editor.state.selection.from,
    to: editor.state.selection.to,
    selection: editor.state.selection.getBookmark(),
  }));
  const state = useEditorState({
    editor,
    selector: ({ editor: current }) => ({
      hasSelection: !current.state.selection.empty,
      link: current.isActive("link"),
    }),
  });

  function remember() {
    setError(null);
    setSnapshot({
      doc: editor.state.doc,
      from: editor.state.selection.from,
      to: editor.state.selection.to,
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
  function applyLink() {
    if (!unchanged() || !safeContentURL(url.trim())) return;
    const chain = editor
      .chain()
      .focus()
      .setTextSelection({ from: snapshot.from, to: snapshot.to })
      .extendMarkRange("link")
      .setLink({ href: url.trim() });
    let valid = false;
    chain.command(({ tr }) => {
      valid = fromEditorDoc(tr.doc).ok;
      return valid;
    });
    if (!chain.run() || !valid) {
      setError(t("contentEditor.editBlocked"));
      return;
    }
    setLinking(false);
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
        <Button
          variant="ghost"
          size="sm"
          disabled={!state.hasSelection && !state.link}
          title={t("groups.linkHint")}
          onClick={() => {
            remember();
            setURL(String(editor.getAttributes("link").href ?? ""));
            setLinking(true);
          }}
        >
          <Link aria-hidden="true" />
          {t("groups.link")}
        </Button>
        {state.link ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              editor.chain().focus().extendMarkRange("link").unsetLink().run()
            }
          >
            <Unlink aria-hidden="true" />
            {t("groups.removeLink")}
          </Button>
        ) : null}
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
      <ConfirmDialog
        open={linking}
        onOpenChange={setLinking}
        title={t("groups.link")}
        description={t("groups.linkHint")}
        confirmLabel={t("common.save")}
        disabled={!safeContentURL(url.trim())}
        onConfirm={applyLink}
        error={error}
      >
        <Field>
          <FieldLabel htmlFor="group-link-url">{t("groups.linkURL")}</FieldLabel>
          <Input
            id="group-link-url"
            type="url"
            value={url}
            onChange={(event) => setURL(event.target.value)}
          />
          <FieldDescription>{t("groups.linkHTTPS")}</FieldDescription>
        </Field>
      </ConfirmDialog>
    </>
  );
}
