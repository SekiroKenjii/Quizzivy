import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { AutosaveStatusLabel } from "@/features/tests/components/AutosaveStatusLabel";
import { useAutosave } from "@/features/tests/useAutosave";
import { setAttemptNote } from "../api";

/** G-05's "Ghi chú của bạn": autosaved like the builder, read by nobody else. */
export function TeacherNoteCard({
  attemptId,
  note,
}: Readonly<{
  attemptId: string;
  note: string | null;
}>) {
  const { t } = useTranslation();
  const [value, setValue] = useState(note ?? "");
  const autosave = useAutosave<string>({
    save: async (next) => {
      await setAttemptNote(attemptId, next.trim() === "" ? null : next);
    },
  });

  return (
    <Card className="gap-0">
      <CardHeader className="flex items-center justify-between">
        <CardTitle>{t("timeline.noteTitle")}</CardTitle>
        <AutosaveStatusLabel status={autosave.status} onRetry={autosave.retry} />
      </CardHeader>
      <CardContent className="space-y-2 pt-3">
        <Textarea
          aria-label={t("timeline.noteTitle")}
          placeholder={t("timeline.notePlaceholder")}
          value={value}
          className="min-h-24"
          onChange={(event) => {
            setValue(event.target.value);
            autosave.schedule(event.target.value);
          }}
        />
        <p className="text-muted-foreground text-xs">{t("timeline.noteHint")}</p>
      </CardContent>
    </Card>
  );
}
