import type { components } from "@/lib/api/schema";
import type { MediaAsset } from "@/features/media/api";
import type { GroupBundle } from "./api";

type AudioPolicy = components["schemas"]["AudioPolicy"];
type StudentAudioPolicy = components["schemas"]["StudentAudioPolicy"];

/** studentAudioPolicy gives a teacher's audio policy the shape a student's engine reads; a policy that does not say allows pausing, as the server's reads do. */
export function studentAudioPolicy(policy: AudioPolicy): StudentAudioPolicy {
  return { ...policy, allowPause: policy.allowPause ?? true };
}

/** groupPreview projects editable teacher content into the learner renderer without keys, explanations or transcripts; preview-only option identities are stable within each question. */
export function groupPreview(bundle: GroupBundle, assets: MediaAsset[]) {
  const { group } = bundle;
  const catalog = new Map(
    bundle.questions.map((question) => [question.id, question.input]),
  );
  const media = new Map(assets.map((asset) => [asset.id, asset]));
  const questions: components["schemas"]["StudentQuestion"][] = group.members.flatMap(
    (member) => {
      const input = catalog.get(member.questionId);
      if (!input) return [];
      return [
        {
          id: member.questionId,
          sectionId: group.id,
          type: input.type,
          prompt: input.prompt,
          promptContent: input.promptContent ?? null,
          points: input.points,
          media: media.get(input.mediaAssetId ?? "") ?? null,
          mediaAlt: input.mediaAlt ?? null,
          audio: input.audio ? studentAudioPolicy(input.audio) : null,
          options: (input.options ?? []).map((option, index) => ({
            id: option.id ?? `${member.questionId}-option-${index}`,
            text: option.text,
            content: option.content ?? null,
          })),
          blanks: (input.blanks ?? []).map((blank) => ({
            id: blank.id ?? `${member.questionId}-blank-${blank.ordinal}`,
            ordinal: blank.ordinal,
            gapId: blank.gapId ?? null,
            caseSensitive: blank.caseSensitive ?? false,
          })),
        },
      ];
    },
  );
  const context: components["schemas"]["StudentGroup"] = {
    id: group.id,
    sectionId: group.id,
    title: group.title,
    instructions: group.instructions ?? null,
    questionIds: questions.map((question) => question.id),
    stimuli: group.stimuli.map((material) => ({
      id: material.id,
      title: material.title,
      content: material.content,
      gaps: material.gaps,
    })),
    recordings: group.recordings.map((recording) => ({
      id: recording.id,
      assetId: recording.assetId,
      policy: studentAudioPolicy(recording.policy),
    })),
    assets,
  };
  return { questions, groups: [context] };
}
