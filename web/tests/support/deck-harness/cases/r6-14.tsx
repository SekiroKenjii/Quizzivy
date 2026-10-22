import { useState, type ReactElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { QuestionEditor } from "@/features/question-bank/components/QuestionEditor";
import {
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { QuestionSheet } from "@/features/take-test/components/QuestionSheet";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { ReviewItem } from "@/features/results/components/ReviewItem";
import type { MediaAsset } from "@/features/media/api";
import type { StudentQuestion } from "@/features/take-test/api";
import type { ResultQuestion } from "@/features/results/api";

const CHART_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360" viewBox="0 0 640 360"><rect width="640" height="360" fill="#fff"/><g fill="#4f6bed"><rect x="80" y="160" width="80" height="160"/><rect x="200" y="100" width="80" height="220"/><rect x="320" y="60" width="80" height="260"/><rect x="440" y="200" width="80" height="120"/></g><line x1="60" y1="320" x2="580" y2="320" stroke="#333" stroke-width="2"/><text x="320" y="36" text-anchor="middle" font-family="sans-serif" font-size="20" fill="#333">Visitors to the museum, 2020-2023</text></svg>`;

const CHART: MediaAsset = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  kind: "image",
  url: `data:image/svg+xml;utf8,${encodeURIComponent(CHART_SVG)}`,
  mimeType: "image/png",
  bytes: 2048,
  originalFilename: "museum-chart.png",
  createdAt: "2026-10-01T00:00:00Z",
};

const RECORDING: MediaAsset = {
  id: "018f0000-0000-7000-8000-0000000000c3",
  kind: "audio",
  url: "data:audio/mpeg;base64,",
  mimeType: "audio/mpeg",
  bytes: 4096,
  durationMs: 94000,
  originalFilename: "part-1-conversation.mp3",
  createdAt: "2026-10-01T00:00:00Z",
};

const MODEL_ANSWER: MediaAsset = {
  ...RECORDING,
  id: "018f0000-0000-7000-8000-0000000000c4",
  durationMs: 41000,
  originalFilename: "model-answer.mp3",
};

const text = (value: string) => ({
  type: "paragraph" as const,
  content: [{ type: "text" as const, text: value, marks: [] }],
});

const LEAD = "Listen to the conversation, then look at the chart.";
const ASK = "Which year had the most visitors?";
const ALT = "Bar chart of museum visitors from 2020 to 2023; 2022 is the highest";
const PROMPT = {
  format: "semantic_v1" as const,
  blocks: [
    text(LEAD),
    { type: "audio" as const },
    { type: "image" as const, assetId: CHART.id, alt: ALT },
    text(ASK),
  ],
};
const PROMPT_TEXT = `${LEAD}\n\n${ALT}\n\n${ASK}`;
const WHY = "2022 has the tallest bar. Hear a model answer:";
const EXPLANATION = {
  format: "semantic_v1" as const,
  blocks: [text(WHY), { type: "audio" as const, assetId: MODEL_ANSWER.id }],
};
const OPTIONS = [
  { id: "018f0000-0000-7000-8000-0000000000e1", text: "2020" },
  { id: "018f0000-0000-7000-8000-0000000000e2", text: "2021" },
  { id: "018f0000-0000-7000-8000-0000000000e3", text: "2022" },
  { id: "018f0000-0000-7000-8000-0000000000e4", text: "2023" },
];

const STUDENT: StudentQuestion = {
  id: "018f0000-0000-7000-8000-0000000000d1",
  sectionId: "018f0000-0000-7000-8000-0000000000d2",
  type: "single_choice",
  prompt: PROMPT_TEXT,
  promptContent: PROMPT as never,
  promptAssets: [CHART],
  media: RECORDING,
  audio: {
    maxPlays: 2,
    allowSeek: false,
    allowPause: true,
    showTranscriptAfterSubmit: false,
  },
  points: 1,
  options: OPTIONS,
};

const RESULT: ResultQuestion = {
  ...STUDENT,
  answer: { type: "options", optionIds: [OPTIONS[2]!.id] },
  earned: 1,
  correctOptionIds: [OPTIONS[2]!.id],
  explanation: WHY,
  explanationContent: EXPLANATION as never,
  explanationAssets: [MODEL_ANSWER],
} as unknown as ResultQuestion;

const client = new QueryClient();

export const cases: Record<string, () => ReactElement> = {
  editor: function Editor() {
    const [value, setValue] = useState<QuestionValues>(() => ({
      ...emptyQuestion(),
      prompt: PROMPT_TEXT,
      promptContent: PROMPT as never,
      explanation: WHY,
      explanationContent: EXPLANATION as never,
      mediaAssetId: RECORDING.id,
      audio: {
        maxPlays: 2,
        allowSeek: false,
        allowPause: true,
        showTranscriptAfterSubmit: false,
      },
      options: OPTIONS.map((option, index) => ({
        id: option.id,
        text: option.text,
        content: null,
        isCorrect: index === 2,
      })) as QuestionValues["options"],
    }));
    const [asset, setAsset] = useState<MediaAsset | null>(RECORDING);
    return (
      <QueryClientProvider client={client}>
        <QuestionEditor
          value={value}
          asset={asset}
          assets={new Map([CHART, MODEL_ANSWER].map((file) => [file.id, file]))}
          onChange={setValue}
          onAssetChange={setAsset}
        />
      </QueryClientProvider>
    );
  },
  engine: () => (
    <div className="bg-card rounded-xl border p-5">
      <QuestionSheet
        question={STUDENT}
        number={3}
        total={12}
        audio={
          <AudioPlayer
            layout="card"
            src={RECORDING.url}
            label="Question audio"
            durationMs={RECORDING.durationMs}
            allowSeek={false}
            plays={{ played: 0, limit: 2 }}
          />
        }
        answer={undefined}
        onAnswer={() => undefined}
      />
    </div>
  ),
  result: () => (
    <ReviewItem
      question={RESULT}
      number={3}
      review={{
        showScore: true,
        showCorrectAnswers: true,
        showExplanations: true,
        release: "on_submit",
        showClassAverage: false,
      }}
      onRetry={() => undefined}
    />
  ),
};
