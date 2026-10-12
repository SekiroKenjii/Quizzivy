import { useTranslation } from "react-i18next";
import { GroupMaterials } from "@/components/shared/content/GroupMaterials";
import { materialsHead } from "@/components/shared/content/materials";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { PassageBody } from "@/features/take-test/components/PassageBody";
import type { components } from "@/lib/api/schema";

type StudentGroup = components["schemas"]["StudentGroup"];

/**
 * PreviewGroupContext shows one frozen context as the engine's stimulus pane
 * draws it, with the group's title as the kicker over the heading of its
 * first material that has something to read, and links its gaps to the
 * displayed questions.
 */
export function PreviewGroupContext({
  group,
  numbers,
  questionAnchor,
  onRetryMedia,
}: Readonly<{
  group: StudentGroup;
  numbers: ReadonlyMap<string, number>;
  questionAnchor: (id: string) => string;
  onRetryMedia?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const head = materialsHead(group);
  return (
    <div className="bg-bg min-w-0 rounded-xl border px-4 py-4.5">
      <PassageBody heading="h3" kicker={head.kicker} title={head.heading}>
        <GroupMaterials
          group={group}
          omitTitle={head.heading}
          onRetryMedia={onRetryMedia}
          renderAudio={(node, asset, recording) => (
            <AudioPlayer
              key={recording.id}
              src={asset.url}
              label={node.label}
              durationMs={asset.durationMs}
              allowSeek={recording.policy.allowSeek}
              hint={t("preview.sharedAudioHint")}
              onRetry={onRetryMedia}
            />
          )}
          renderGap={(binding, label) => {
            const number = numbers.get(binding.questionId);
            if (number == null) return <span className="content-gap">{label}</span>;
            const anchor = questionAnchor(binding.questionId);
            return (
              <a
                className="content-gap"
                href={`#${anchor}`}
                aria-label={t("preview.goToQuestion", { n: number, label })}
                onClick={() => document.getElementById(anchor)?.focus()}
              >
                {label}
              </a>
            );
          }}
        />
      </PassageBody>
    </div>
  );
}
