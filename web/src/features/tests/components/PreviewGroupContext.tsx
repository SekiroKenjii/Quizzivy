import { useTranslation } from "react-i18next";
import { GroupMaterials } from "@/components/shared/content/GroupMaterials";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { components } from "@/lib/api/schema";

type StudentGroup = components["schemas"]["StudentGroup"];

/** PreviewGroupContext shows one frozen context and links its gaps to the displayed questions. */
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
  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>
          <h3>{group.title}</h3>
        </CardTitle>
        <CardDescription>
          {t("preview.sharedRange", {
            from: numbers.get(group.questionIds[0] ?? ""),
            to: numbers.get(group.questionIds.at(-1) ?? ""),
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-5">
        <GroupMaterials
          group={group}
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
      </CardContent>
    </Card>
  );
}
