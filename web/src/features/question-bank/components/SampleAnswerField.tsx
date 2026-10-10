import { useTranslation } from "react-i18next";
import { Textarea } from "@/components/ui/textarea";

/** SampleAnswerField is a short-answer question's optional model answer, shown to graders. */
export function SampleAnswerField({
  value,
  onChange,
}: Readonly<{ value: string | null; onChange: (value: string) => void }>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-1.5">
      <label
        htmlFor="question-sample-answer"
        className="text-[12.5px] font-medium in-data-[field-size=page]:text-[13px] in-data-[field-size=page]:leading-[19.5px]"
      >
        {t("questionEditor.sampleAnswer")}
      </label>
      <p id="question-sample-answer-hint" className="text-muted-fg -mt-1 text-[12.5px]">
        {t("questionEditor.sampleAnswerHint")}
      </p>
      <Textarea
        id="question-sample-answer"
        rows={3}
        value={value ?? ""}
        placeholder={t("questionEditor.sampleAnswerPlaceholder")}
        aria-describedby="question-sample-answer-hint"
        className="bg-background min-h-0 resize-y rounded-[9px] px-3 py-2.5 leading-[1.55] lg:text-[14px] in-data-[scale=deck]:lg:text-[14px]"
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
