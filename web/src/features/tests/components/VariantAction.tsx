import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Sparkles } from "lucide-react";
import type { AiVariantChange } from "@/features/ai/api";
import { AiEntryButton } from "@/features/ai/components/AiEntryButton";
import { AiPanelStack } from "@/features/ai/components/AiPanelStack";
import { useAiAvailable, useAiBlock } from "@/features/ai/useAiAvailable";
import type { Test } from "@/features/tests/api";
import { saveVariant } from "@/features/tests/aiVariant";

const TITLE_MAX = 200;

/**
 * VariantAction is Test detail's "Make a variant" (V-12) and its panel. It is
 * not drawn without the assistant or `ai.use`, and is drawn disabled with the
 * reason while the teacher is paused or out of credits (Q34) or the test has
 * no published version. Kept changes become a new draft test (DG-197).
 * Closing the panel keeps its session while the page is open.
 */
export function VariantAction({
  test,
  className,
}: Readonly<{
  test: Pick<Test, "id" | "title" | "currentVersion">;
  className?: string;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const available = useAiAvailable();
  const blocked = useAiBlock(available);
  const [open, setOpen] = useState(false);

  if (!available) return null;

  function variantTitle() {
    const full = t("ai.variant.testTitle", { title: test.title });
    if (full.length <= TITLE_MAX) return full;
    const room = TITLE_MAX - (full.length - test.title.length);
    return t("ai.variant.testTitle", { title: test.title.slice(0, room).trimEnd() });
  }

  async function save(changes: readonly AiVariantChange[]) {
    const id = await saveVariant(test, changes, {
      title: variantTitle(),
      invalid: (n) => t("ai.variant.invalid", { n }),
    });
    await client.invalidateQueries({ queryKey: ["teacher-tests"] });
    return id;
  }

  return (
    <>
      <AiEntryButton
        icon={Sparkles}
        label={t("ai.entry.variant")}
        blocked={blocked}
        unavailable={test.currentVersion === 0 ? t("ai.needsVersion") : null}
        className={className}
        onClick={() => setOpen(true)}
      />
      <AiPanelStack
        key={test.id}
        open={open ? "variant" : null}
        requestFor={() => ({
          kind: "variant",
          context: { testId: test.id, title: test.title, save },
        })}
        onClose={() => setOpen(false)}
      />
    </>
  );
}
