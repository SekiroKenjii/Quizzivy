import { contentStringLength } from "@/components/shared/content/unicode";
import { DEFAULT_AUDIO_POLICY } from "@/features/question-bank/audioPolicy";
import type { LibraryAsset, MediaAsset } from "@/features/media/api";
import type { GroupBundle } from "./api";
import { materialAssets, type GroupMaterial, type GroupRecording } from "./model";

/**
 * SharedContent is what the Shared content dialog edits of a group: nothing,
 * one passage (a material with no audio), or one recording (a material whose
 * content is a single audio block, with the recording bound to that asset).
 */
export type SharedContent =
  | { kind: "none" }
  | { kind: "passage"; material: GroupMaterial }
  | { kind: "audio"; material: GroupMaterial; recording: GroupRecording };

/**
 * readSharedContent reads a group's shared content in the dialog's terms, or
 * returns null when the group holds more than the dialog shows: several
 * materials, a material mixing text and audio, several recordings, or a
 * recording no material plays.
 */
export function readSharedContent(bundle: GroupBundle): SharedContent | null {
  const { stimuli, recordings } = bundle.group;
  const [material, ...others] = stimuli;
  if (!material) return recordings.length === 0 ? { kind: "none" } : null;
  if (others.length > 0) return null;
  const audio = audioAssetIds(material);
  if (audio.length === 0)
    return recordings.length === 0 ? { kind: "passage", material } : null;
  const [assetId] = audio;
  const recording = recordings.find((item) => item.assetId === assetId);
  if (audio.length > 1 || recordings.length > 1 || !recording || holdsReading(material))
    return null;
  return { kind: "audio", material, recording };
}

/**
 * holdsReading says whether a material has something to read besides its
 * audio: any block but an audio block or an empty paragraph. A material
 * stored as Markdown always does.
 */
export function holdsReading(material: GroupMaterial): boolean {
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

/** audioAssetIds lists the audio assets a material plays, in its order. */
export function audioAssetIds(material: GroupMaterial): string[] {
  return [...materialAssets([material])].flatMap(([assetId, kind]) =>
    kind === "audio" ? [assetId] : [],
  );
}

/**
 * withSharedContent is `bundle` with its materials and recordings replaced by
 * `content`, and everything else, its title, instructions and members
 * included, left as it is.
 */
export function withSharedContent(
  bundle: GroupBundle,
  content: SharedContent,
): GroupBundle {
  const stimuli = content.kind === "none" ? [] : [content.material];
  const recordings = content.kind === "audio" ? [content.recording] : [];
  return { ...bundle, group: { ...bundle.group, stimuli, recordings } };
}

/**
 * audioContent is the shared content that plays `asset`: one material whose
 * content is the asset's audio block, labelled `label`, and titled with the
 * file's name without its extension, and one recording of it. `previous`, the
 * recording being replaced, keeps its material's id and title when it plays
 * the same asset, and lends its policy and transcript to a new one; with no
 * previous recording, a play limit stored with a library file is the default
 * (DG-69).
 */
export function audioContent(
  asset: MediaAsset | LibraryAsset,
  label: string,
  previous?: Extract<SharedContent, { kind: "audio" }>,
): Extract<SharedContent, { kind: "audio" }> {
  const same = previous?.recording.assetId === asset.id;
  return {
    kind: "audio",
    material: {
      id: previous?.material.id ?? crypto.randomUUID(),
      title:
        same && previous ? previous.material.title : fileTitle(asset.originalFilename),
      content: {
        format: "semantic_v1",
        blocks: [{ type: "audio", assetId: asset.id, label }],
      },
      gaps: [],
    },
    recording: {
      id: previous?.recording.id ?? crypto.randomUUID(),
      assetId: asset.id,
      policy: previous ? previous.recording.policy : startingPolicy(asset),
      transcript: previous ? (previous.recording.transcript ?? null) : null,
    },
  };
}

const TITLE_LIMIT = 200;

/**
 * fileTitle is a file's name without its extension, trimmed and cut to a
 * material title's 200 characters; a name that leaves nothing keeps the
 * whole name.
 */
export function fileTitle(name: string): string {
  const dot = name.lastIndexOf(".");
  const stem = (dot > 0 ? name.slice(0, dot) : name).trim() || name.trim();
  if (contentStringLength(stem) <= TITLE_LIMIT) return stem;
  return Array.from(stem).slice(0, TITLE_LIMIT).join("").trim();
}

function startingPolicy(asset: MediaAsset | LibraryAsset): GroupRecording["policy"] {
  const stored = "defaultMaxPlays" in asset ? asset.defaultMaxPlays : null;
  if (stored === null) return { ...DEFAULT_AUDIO_POLICY };
  return { ...DEFAULT_AUDIO_POLICY, maxPlays: stored === 0 ? null : stored };
}
