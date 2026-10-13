import type { TFunction } from "i18next";
import type { GroupBundle } from "@/features/question-groups/api";
import {
  materialAssets,
  materialGaps,
  type GroupMaterial,
  type GroupRecording,
} from "@/features/question-groups/model";

/**
 * SharedContentLine is the outline's one-line summary of a shared-context
 * group's shared content: its kind, which picks the icon, and its text.
 */
export interface SharedContentLine {
  kind: "passage" | "audio" | "materials";
  text: string;
}

/**
 * sharedContentLine summarises a group's shared content as the deck's
 * outline line does. One material with no recording reads
 * "Passage · {title}", plus its gap count when it has gaps. One material that
 * is a single recording and nothing to read reads
 * "Audio · {file} · {plays}", where `fileNames` maps an asset id to its file
 * name and an unknown name is left out. More than one material, or one that
 * holds text and audio or several recordings, reads
 * "{n} materials · {m} recordings". A group with no material has no line, and
 * the result is null.
 */
export function sharedContentLine(
  bundle: GroupBundle,
  fileNames: ReadonlyMap<string, string>,
  t: TFunction,
): SharedContentLine | null {
  const { stimuli, recordings } = bundle.group;
  const [material, ...others] = stimuli;
  if (!material) return null;
  if (others.length === 0) {
    const audio = [...materialAssets([material])].flatMap(([assetId, kind]) =>
      kind === "audio" ? [assetId] : [],
    );
    if (audio.length === 0) return passageLine(material, t);
    const [assetId] = audio;
    if (audio.length === 1 && assetId && !holdsReading(material))
      return audioLine(
        fileNames.get(assetId),
        recordings.find((recording) => recording.assetId === assetId),
        t,
      );
  }
  return {
    kind: "materials",
    text: [
      t("builder.sharedMaterials", { count: stimuli.length }),
      t("builder.sharedRecordings", { count: recordings.length }),
    ].join(SEPARATOR),
  };
}

const SEPARATOR = " · ";

function passageLine(material: GroupMaterial, t: TFunction): SharedContentLine {
  const gaps = materialGaps(material).length;
  return {
    kind: "passage",
    text: [
      t("builder.sharedPassage"),
      material.title.trim() || t("builder.sharedUntitled"),
      ...(gaps > 0 ? [t("builder.sharedGaps", { count: gaps })] : []),
    ].join(SEPARATOR),
  };
}

function audioLine(
  fileName: string | undefined,
  recording: GroupRecording | undefined,
  t: TFunction,
): SharedContentLine {
  const maxPlays = recording?.policy.maxPlays;
  return {
    kind: "audio",
    text: [
      t("builder.sharedAudio"),
      ...(fileName ? [fileName] : []),
      ...(maxPlays === undefined
        ? []
        : [
            maxPlays === null
              ? t("builder.sharedUnlimitedPlays")
              : t("builder.sharedPlays", { count: maxPlays }),
          ]),
    ].join(SEPARATOR),
  };
}

function holdsReading(material: GroupMaterial): boolean {
  const content = material.content;
  if (content.format !== "semantic_v1") return true;
  return content.blocks.some(
    (block) =>
      block.type !== "audio" &&
      !(
        block.type === "paragraph" &&
        block.content.every(
          (inline) => inline.type === "text" && inline.text.trim() === "",
        )
      ),
  );
}
