import type { components } from "@/lib/api/schema";
import type { MediaAsset } from "@/features/media/api";
import type { AdminQuestion } from "@/features/question-bank/api";
import { matchItemKey, matchOptionKey } from "@/features/question-bank/matchKeys";
import type { GroupBundle } from "./api";

type AudioPolicy = components["schemas"]["AudioPolicy"];
type StudentAudioPolicy = components["schemas"]["StudentAudioPolicy"];

/** studentAudioPolicy gives a teacher's audio policy the shape a student's engine reads; a policy that does not say allows pausing, as the server's reads do. */
export function studentAudioPolicy(policy: AudioPolicy): StudentAudioPolicy {
  return { ...policy, allowPause: policy.allowPause ?? true };
}

type StudentRules = Pick<
  components["schemas"]["StudentQuestion"],
  "selectCount" | "maxWords" | "matchAllowReuse"
>;

/** studentRules copies the rules a student is told (choose N, the word limit, reusing a matching choice) and leaves out the ones a question does not set. */
export function studentRules(question: {
  selectCount?: number | null;
  maxWords?: number | null;
  matchAllowReuse?: boolean | null;
}): StudentRules {
  return {
    ...(question.selectCount != null ? { selectCount: question.selectCount } : {}),
    ...(question.maxWords != null ? { maxWords: question.maxWords } : {}),
    ...(question.matchAllowReuse != null
      ? { matchAllowReuse: question.matchAllowReuse }
      : {}),
  };
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
            ...(input.type === "matching" ? { key: matchOptionKey(index) } : {}),
          })),
          blanks: (input.blanks ?? []).map((blank) => ({
            id: blank.id ?? `${member.questionId}-blank-${blank.ordinal}`,
            ordinal: blank.ordinal,
            gapId: blank.gapId ?? null,
            caseSensitive: blank.caseSensitive ?? false,
          })),
          ...studentRules(input),
          ...(input.matchItems
            ? {
                matchItems: input.matchItems.map((item, index) => ({
                  id: item.id ?? `${member.questionId}-item-${index}`,
                  key: matchItemKey(index),
                  text: item.text,
                })),
              }
            : {}),
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

/**
 * asStudent gives a draft question the shape a student receives, §13.5's
 * boundary applied client-side: the rules a student is told (choose N, the
 * word limit, reuse, the matching items) stay, and nothing that grades
 * (`isCorrect`, accepted answers, an item's match, the rubric) survives.
 */
export function asStudent(
  sectionId: string,
  q: AdminQuestion,
): components["schemas"]["StudentQuestion"] {
  return {
    id: q.id,
    sectionId,
    type: q.type,
    prompt: q.prompt,
    promptContent: q.promptContent ?? null,
    points: q.points,
    media: q.media ?? null,
    audio: q.audio ? studentAudioPolicy(q.audio) : null,
    options: (q.options ?? []).map((o) => ({
      id: o.id,
      text: o.text,
      content: o.content ?? null,
      ...(q.type === "matching" ? { key: o.key ?? matchOptionKey(o.ordinal) } : {}),
    })),
    blanks: (q.blanks ?? []).map((b) => ({
      id: b.id,
      ordinal: b.ordinal,
      gapId: b.gapId ?? null,
      caseSensitive: b.caseSensitive ?? false,
    })),
    ...studentRules(q),
    ...(q.matchItems
      ? {
          matchItems: q.matchItems.map(({ id, key, text }) => ({
            id,
            key,
            text,
          })),
        }
      : {}),
  };
}
