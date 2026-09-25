import { useRef } from "react";
import { useTranslation } from "react-i18next";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { ContentView } from "@/components/shared/content/ContentView";
import { ContentImage } from "@/components/shared/content/ContentImage";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { MediaAsset } from "@/features/media/api";
import {
  changeMaterialContent,
  materialGaps,
  type GroupGapBinding,
  type GroupMaterial,
} from "../model";
import { MaterialTools } from "./MaterialTools";

export function MaterialContent({
  material,
  preview,
  assets,
  onChange,
  onAsset,
  onRefresh,
}: Readonly<{
  material: GroupMaterial;
  preview: boolean;
  assets: ReadonlyMap<string, MediaAsset>;
  onChange: (material: GroupMaterial) => void;
  onAsset: (asset: MediaAsset) => void;
  onRefresh: () => void;
}>) {
  const { t } = useTranslation();
  const bindings = useRef(new Map<string, GroupGapBinding>());
  return (
    <>
      <div hidden={preview}>
        {material.content.format === "semantic_v1" ? (
          <ContentEditor
            key={material.id}
            initialContent={material.content}
            label={t("groups.materialContent")}
            onChange={(content) => {
              const active = new Set(material.gaps.map((binding) => binding.gapId));
              for (const gap of materialGaps(material))
                if (!active.has(gap.id)) bindings.current.delete(gap.id);
              for (const binding of material.gaps)
                bindings.current.set(binding.gapId, binding);
              const next = changeMaterialContent(material, content);
              next.gaps = materialGaps(next).flatMap((gap) => {
                const binding = bindings.current.get(gap.id);
                return binding ? [binding] : [];
              });
              onChange(next);
            }}
            tools={(editor) => <MaterialTools editor={editor} onAsset={onAsset} />}
          />
        ) : (
          <Field>
            <FieldLabel htmlFor={`legacy-material-${material.id}`}>
              {t("groups.materialContent")}
            </FieldLabel>
            <Textarea
              id={`legacy-material-${material.id}`}
              value={material.content.markdown}
              maxLength={100_000}
              className="min-h-64"
              onChange={(event) =>
                onChange({
                  ...material,
                  content: {
                    format: "legacy_markdown_v1",
                    markdown: event.target.value,
                  },
                })
              }
            />
            <FieldDescription>{t("groups.legacyMaterial")}</FieldDescription>
          </Field>
        )}
      </div>
      {preview ? (
        <ContentView
          document={material.content}
          className="rounded-lg border p-5"
          renderAsset={(node) => {
            const asset = assets.get(node.assetId);
            if (!asset)
              return (
                <Button variant="outline" onClick={onRefresh}>
                  {t("groups.retryAsset")}
                </Button>
              );
            return node.type === "image" ? (
              <ContentImage src={asset.url} alt={node.alt} onRetry={onRefresh} />
            ) : (
              <AudioPlayer
                src={asset.url}
                label={node.label}
                durationMs={asset.durationMs}
                onRetry={onRefresh}
              />
            );
          }}
        />
      ) : null}
    </>
  );
}
