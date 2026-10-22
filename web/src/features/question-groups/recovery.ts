import { z } from "zod";
import { questionGaps } from "@/components/shared/content/gaps";
import type {
  ContentDocument,
  SemanticContent,
} from "@/components/shared/content/model";
import { validateContent } from "@/components/shared/content/validation";
import {
  questionContentSchema,
  questionPromptContentSchema,
} from "@/components/shared/content/questionContent";
import { optionContentSchema } from "@/components/shared/content/optionContent";
import type { GroupBundle } from "./api";
import { formProse } from "@/components/shared/content/questionContent";

const policy = z.object({
  maxPlays: z.number().int().min(1).nullable(),
  allowSeek: z.boolean(),
  showTranscriptAfterSubmit: z.boolean(),
  allowPause: z.boolean().exactOptional(),
});
const question = z.object({
  type: z.enum([
    "single_choice",
    "multiple_choice",
    "true_false",
    "fill_blank",
    "short_answer",
  ]),
  level: z
    .enum(["pre_a1", "a1", "a2", "b1", "b2", "c1", "c2"])
    .nullable()
    .default(null),
  skill: z
    .enum(["grammar", "vocabulary", "reading", "listening", "writing", "speaking"])
    .nullable()
    .default(null),
  prompt: z.string(),
  points: z.number(),
  promptContent: questionPromptContentSchema.nullish(),
  explanationContent: questionContentSchema.nullish(),
  mediaAssetId: z.uuid().nullish(),
  mediaAlt: z.string().nullish(),
  audio: policy.nullish(),
  transcript: z.string().nullish(),
  explanation: z.string().nullish(),
  sampleAnswer: z.string().nullish(),
  tags: z.array(z.string()).optional(),
  options: z
    .array(
      z.object({
        id: z.uuid().nullish(),
        text: z.string(),
        isCorrect: z.boolean(),
        content: optionContentSchema.nullish(),
      }),
    )
    .optional(),
  blanks: z
    .array(
      z.object({
        id: z.uuid().nullish(),
        gapId: z.string().nullish(),
        ordinal: z.number(),
        acceptedAnswers: z.array(z.string()),
        caseSensitive: z.boolean(),
      }),
    )
    .optional(),
});
const binding = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("question"), gapId: z.string(), questionId: z.uuid() }),
  z.object({
    kind: z.literal("blank"),
    gapId: z.string(),
    questionId: z.uuid(),
    blankGapId: z.string(),
  }),
]);
const bundleSchema = z.object({
  group: z.object({
    id: z.uuid(),
    title: z.string(),
    instructions: questionContentSchema.nullish(),
    members: z
      .array(
        z.object({ questionId: z.uuid(), optionOrder: z.enum(["shuffle", "fixed"]) }),
      )
      .max(200),
    stimuli: z
      .array(
        z.object({
          id: z.uuid(),
          title: z.string(),
          content: z.custom<ContentDocument>((value) => validateContent(value).ok),
          gaps: z.array(binding),
        }),
      )
      .max(16),
    recordings: z
      .array(
        z.object({
          id: z.uuid(),
          assetId: z.uuid(),
          policy,
          transcript: z.string().nullish(),
        }),
      )
      .max(16),
  }),
  questions: z.array(z.object({ id: z.uuid(), input: question })).max(200),
});
const recoverySchema = z.object({
  version: z.literal(1),
  revision: z.number().int().min(1),
  testUpdatedAt: z.string().nullish(),
  bundle: bundleSchema,
});

export interface GroupRecovery {
  version: 1;
  revision: number;
  testUpdatedAt?: string | null | undefined;
  bundle: GroupBundle;
}

/** readGroupRecovery accepts incomplete edits but rejects malformed or unrelated local payloads before mounting an editor. */
export function readGroupRecovery(
  value: unknown,
  groupId: string,
): GroupRecovery | null {
  try {
    if (new TextEncoder().encode(JSON.stringify(value)).byteLength > 4 * 1024 * 1024)
      return null;
  } catch {
    return null;
  }
  const parsed = recoverySchema.safeParse(value);
  if (!parsed.success || parsed.data.bundle.group.id !== groupId) return null;
  const bundle = parsed.data.bundle as GroupBundle;
  const ids = new Set(bundle.questions.map((item) => item.id));
  if (
    ids.size !== bundle.questions.length ||
    ids.size !== bundle.group.members.length ||
    bundle.group.members.some((member) => !ids.delete(member.questionId))
  )
    return null;
  return { ...parsed.data, bundle };
}

type AudioPolicy = GroupBundle["group"]["recordings"][number]["policy"];

/**
 * withStoredPause gives each audio policy of a recovered draft that has no
 * `allowPause` the value `stored` holds for the same recording or member
 * question, by id, and true where `stored` holds none. A draft written before
 * the field existed therefore restores the rule last saved, and cannot switch
 * pausing back on. A policy that has the field keeps it.
 */
export function withStoredPause(
  recovery: GroupRecovery,
  stored: GroupBundle,
): GroupRecovery {
  const recordings = new Map(
    stored.group.recordings.map((recording) => [
      recording.id,
      recording.policy.allowPause,
    ]),
  );
  const questions = new Map(
    stored.questions.map((question) => [question.id, question.input.audio?.allowPause]),
  );
  const { group, questions: members } = recovery.bundle;
  return {
    ...recovery,
    bundle: {
      group: {
        ...group,
        recordings: group.recordings.map((recording) => ({
          ...recording,
          policy: withPause(recording.policy, recordings.get(recording.id)),
        })),
      },
      questions: members.map((question) =>
        question.input.audio
          ? {
              ...question,
              input: {
                ...question.input,
                audio: withPause(question.input.audio, questions.get(question.id)),
              },
            }
          : question,
      ),
    },
  };
}

function withPause(policy: AudioPolicy, stored: boolean | undefined): AudioPolicy {
  if (policy.allowPause !== undefined) return policy;
  return { ...policy, allowPause: stored ?? true };
}

/** independentBundle remaps graph identities while preserving material gap and rich blank links for recovery as a new bank group. */
export function independentBundle(source: GroupBundle): GroupBundle {
  const bundle = structuredClone(source);
  const ids = new Map(
    bundle.questions.map((question) => [question.id, crypto.randomUUID()]),
  );
  const blankGaps = new Map<string, Map<string, string>>();
  const questions = bundle.questions.map((question) => {
    const input = question.input;
    const gaps = input.promptContent
      ? remapGaps(formProse(input.promptContent)!)
      : new Map<string, string>();
    blankGaps.set(question.id, gaps);
    return {
      id: ids.get(question.id)!,
      input: {
        ...input,
        options: (input.options ?? []).map((option) => ({ ...option, id: null })),
        blanks: (input.blanks ?? []).map((blank) => ({
          ...blank,
          id: null,
          gapId: blank.gapId ? (gaps.get(blank.gapId) ?? null) : null,
        })),
      },
    };
  });
  return {
    group: {
      ...bundle.group,
      id: crypto.randomUUID(),
      members: bundle.group.members.map((member) => ({
        ...member,
        questionId: ids.get(member.questionId)!,
      })),
      stimuli: bundle.group.stimuli.map((material) => {
        const gaps =
          material.content.format === "semantic_v1"
            ? remapGaps(material.content)
            : new Map<string, string>();
        return {
          ...material,
          id: crypto.randomUUID(),
          gaps: material.gaps.map((binding) => ({
            ...binding,
            questionId: ids.get(binding.questionId)!,
            gapId: gaps.get(binding.gapId)!,
            ...(binding.kind === "blank"
              ? {
                  blankGapId: blankGaps
                    .get(binding.questionId)!
                    .get(binding.blankGapId)!,
                }
              : {}),
          })),
        };
      }),
      recordings: bundle.group.recordings.map((recording) => ({
        ...recording,
        id: crypto.randomUUID(),
      })),
    },
    questions,
  };
}

function remapGaps(document: SemanticContent): Map<string, string> {
  const ids = new Map<string, string>();
  for (const gap of questionGaps(document)) {
    const id = crypto.randomUUID();
    ids.set(gap.id, id);
    gap.id = id;
  }
  return ids;
}
