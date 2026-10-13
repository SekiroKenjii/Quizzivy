import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import type { MaterialGap } from "@/components/shared/content/GroupMaterials";
import { hasReading } from "@/components/shared/content/materials";
import { FullscreenBar } from "@/features/integrity/components/FullscreenBar";
import { cn } from "@/lib/utils";
import { ClozeBody } from "./ClozeBody";
import { EngineHeader, LeaveButton } from "./EngineHeader";
import { GroupContext, type DrawerPane } from "./GroupContext";
import { Navigator, type KeysHint } from "./Navigator";
import { QuestionCard } from "./QuestionCard";
import { SaveStrip } from "./SaveState";
import { SectionRail } from "./SectionRail";
import { DrawerBackdrop, DrawerHandle, DrawerTip } from "./StimulusDrawer";
import { dotsOf } from "../answered";
import type { StudentQuestion } from "../api";
import {
  hasSharedContent,
  useKeptScroll,
  useSharedAudioPlaying,
  useStimulusDrawer,
} from "../panes";
import { usePaper } from "../paper";
import { clozeLine, questionKind } from "../questionType";
import { groupBySection, sectionAt, sectionHead, type SectionHead } from "../sections";
import { isRun, paperStops, stepBack, stepOn, type Stop } from "../stops";
import { useEngineWidthAtLeast } from "../width";

const WIDE_PX = 768;
const RAIL_PX = 900;
const RAIL_BESIDE_SHARED_PX = 1180;
const SHORT_LABEL_PX = 420;

/**
 * PaperProps is what the host of a paper decides: the attempt, under which
 * the drawers a student has opened are remembered (none, in memory only),
 * the open question, the strike count, the fullscreen bar, and what each
 * control does.
 */
export type PaperProps = Readonly<{
  attemptId?: string | undefined;
  index: number;
  jumps: number;
  status: ReactNode;
  fullscreenBar: boolean;
  onLeave: () => void;
  onMove: (index: number) => void;
  onJump: (index: number) => void;
  onSubmit: () => void;
  onReload: () => void;
}>;

/**
 * Paper is the engine's paper open at question `index`: the header, the
 * sections rail, the stimulus pane and the question pane with the navigator.
 * The question pane is on the sidebar surface, its column at most 600px,
 * beside shared content from 768px, and on the page's background, its column
 * at most 720px, alone or on a phone. Above every question the column states
 * the section's title and instructions, as sectionHead decides.
 * The rail shows when the engine is at least 900px wide on a question with no
 * shared content and at least 1180px on one with it, and the footer then
 * names the keys; otherwise the footer's grid button opens the question
 * sheet, and its label takes the short form beside shared content from 768px
 * and below 420px. It reads the paper from the nearest PaperSourceContext and
 * the width from useEngineWidthAtLeast, and draws nothing for a paper with no
 * questions. A run of a cloze is one stop: ClozeBody draws it, open on the
 * gap at `index`, Previous and Next step over it, and a gap opened inside it
 * keeps the column's scroll and focus; a gap of a cloze shown on its own
 * says so in its line.
 */
export function Paper(props: PaperProps) {
  const { questions } = usePaper();
  const question = questions[Math.min(props.index, questions.length - 1)];
  if (question === undefined) return null;
  return <PaperAt {...props} question={question} />;
}

function PaperAt({
  attemptId,
  index,
  jumps,
  question,
  status,
  fullscreenBar,
  onLeave,
  onMove,
  onJump,
  onSubmit,
  onReload,
}: PaperProps & Readonly<{ question: StudentQuestion }>) {
  const { t } = useTranslation();
  const wide = useEngineWidthAtLeast(WIDE_PX);
  const railFits = useEngineWidthAtLeast(RAIL_PX);
  const railFitsBesideShared = useEngineWidthAtLeast(RAIL_BESIDE_SHARED_PX);
  const narrow = !useEngineWidthAtLeast(SHORT_LABEL_PX);
  const {
    questions,
    sections,
    groups: sharedGroups,
    answers,
    flags,
    lock,
  } = usePaper();
  const sealed = lock === "superseded" || lock === "closed";
  const submit = sealed ? undefined : onSubmit;
  const groups = useMemo(
    () => groupBySection(sections, questions),
    [sections, questions],
  );
  const group = sectionAt(groups, index);
  const dots = dotsOf(questions, answers, flags);
  const total = questions.length;
  const answerPanel = useRef<HTMLDivElement>(null);
  const contexts = useMemo(
    () =>
      new Map(
        sharedGroups.flatMap((context) =>
          context.questionIds.map((id) => [id, context] as const),
        ),
      ),
    [sharedGroups],
  );
  const numbers = useMemo(
    () => new Map(questions.map((item, i) => [item.id, i + 1])),
    [questions],
  );
  const stops = useMemo(
    () => paperStops(questions, sharedGroups),
    [questions, sharedGroups],
  );
  const stop = stops[index] ?? { positions: [index], cloze: null };
  const screen = questions[stop.positions[0] ?? index]?.id ?? question.id;
  const context = contexts.get(question.id);
  const passage = hasSharedContent(context) ? context : undefined;
  const visit = `${screen}:${jumps}`;
  const drawerMode = !wide && passage !== undefined;
  const panes = useRef<HTMLElement>(null);
  const playing = useSharedAudioPlaying(panes);
  const {
    open: drawerOpen,
    out: drawerOut,
    offset: drawerOffset,
    progress: drawerProgress,
    fresh,
    show: showDrawer,
    close: closeDrawer,
    dismiss,
    handlers: swipeHandlers,
    handleRef,
    closeRef,
  } = useStimulusDrawer({
    root: panes,
    enabled: drawerMode,
    visit,
    groupId: passage?.id,
    attemptId,
  });
  const sheet = useRef<HTMLDivElement>(null);
  const onSheetScroll = useKeptScroll(sheet, false, screen);
  const landOn = useLanding(visit, sheet, answerPanel);

  const jumpToGap = (gap: MaterialGap) => {
    const landing = gapLanding(gap, questions, numbers);
    if (landing === null) return;
    dismiss();
    landOn(landing.target);
    onJump(landing.index);
  };
  const drawer: DrawerPane | undefined = drawerMode
    ? {
        open: drawerOpen,
        offset: drawerOffset,
        closeRef,
        onClose: closeDrawer,
      }
    : undefined;
  const kind = passage?.stimuli.some(hasReading) === false ? "recording" : "passage";
  const head = sectionHead(groups, group);
  const besideShared = wide && passage !== undefined;
  const rail = wide && (passage === undefined ? railFits : railFitsBesideShared);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <EngineHeader
        wide={wide}
        status={status}
        leading={<LeaveButton onClick={onLeave} />}
        onSubmit={submit}
      />
      <SaveStrip wide={wide} indicator={status} />
      {fullscreenBar && <FullscreenBar />}

      <main
        ref={panes}
        tabIndex={-1}
        aria-label={t("takeTest.dotLabel", { n: index + 1 })}
        className="relative flex min-h-0 min-w-0 flex-1 outline-none!"
        style={drawerMode ? { touchAction: "pan-y" } : undefined}
        {...swipeHandlers}
      >
        {rail && (
          <SectionRail
            dots={dots}
            current={index}
            groups={groups}
            stops={stops}
            onJump={onJump}
          />
        )}
        {passage !== undefined && (
          <GroupContext
            key={passage.id}
            group={passage}
            sectionTitle={group?.section.title}
            numbers={numbers}
            openId={question.id}
            onGap={jumpToGap}
            onRetryMedia={onReload}
            wide={wide}
            drawer={drawer}
          />
        )}
        <section
          className={cn(
            "flex min-w-0 flex-[1_1_0] flex-col",
            besideShared ? "bg-sidebar" : "bg-bg",
          )}
        >
          <div
            ref={sheet}
            onScroll={onSheetScroll}
            className={cn(
              "relative min-h-0 flex-1 overflow-y-auto",
              wide ? "px-8 py-7" : "px-4 py-4.5",
              drawerMode && "touch-pan-y",
            )}
          >
            <div
              ref={answerPanel}
              id={`answer-question-${question.id}`}
              tabIndex={-1}
              aria-label={t("takeTest.dotLabel", { n: index + 1 })}
              className={cn(
                "mx-auto flex w-full flex-col gap-4.5 outline-none!",
                passage === undefined ? "max-w-180" : "max-w-150",
              )}
            >
              {head && <SectionHeader head={head} />}
              <StopBody
                stop={stop}
                question={question}
                index={index}
                total={total}
                onMove={onMove}
                onReload={onReload}
              />
            </div>
          </div>
          <Navigator
            wide={wide}
            rail={rail}
            hint={keysHintOf(question, passage !== undefined)}
            short={narrow || (wide && passage !== undefined)}
            dots={dots}
            current={index}
            groups={groups}
            stops={stops}
            back={stepBack(stops, index)}
            ahead={stepOn(stops, index)}
            onMove={onMove}
            onJump={onJump}
            onFinish={submit}
          />
        </section>
        {drawerMode && drawerOut && (
          <DrawerBackdrop progress={drawerProgress} onClose={closeDrawer} />
        )}
        {drawerMode && !drawerOut && (
          <DrawerHandle
            ref={handleRef}
            kind={kind}
            playing={playing}
            nudge={fresh}
            onOpen={showDrawer}
          />
        )}
        {drawerMode && !drawerOut && fresh && <DrawerTip kind={kind} />}
      </main>
    </div>
  );
}

function StopBody({
  stop,
  question,
  index,
  total,
  onMove,
  onReload,
}: Readonly<{
  stop: Stop;
  question: StudentQuestion;
  index: number;
  total: number;
  onMove: (index: number) => void;
  onReload: () => void;
}>) {
  const { t } = useTranslation();
  if (isRun(stop)) {
    return (
      <ClozeBody
        stop={stop}
        cloze={stop.cloze}
        open={index}
        total={total}
        onOpen={onMove}
      />
    );
  }
  return (
    <QuestionCard
      question={question}
      number={index + 1}
      total={total}
      line={
        stop.cloze === null
          ? undefined
          : clozeLine(stop.positions, stop.cloze.positions.length, total, t)
      }
      namesFlag={stop.cloze !== null}
      onAudioExpired={onReload}
    />
  );
}

function SectionHeader({ head }: Readonly<{ head: SectionHead }>) {
  return (
    <div className="flex flex-col gap-0.5 border-b pb-3">
      {head.title !== "" && (
        <p className="text-muted-fg text-xs leading-normal font-semibold tracking-[0.04em] uppercase">
          {head.title}
        </p>
      )}
      {head.instructions !== "" && (
        <p className="text-muted-fg text-ui leading-normal text-pretty">
          {head.instructions}
        </p>
      )}
    </div>
  );
}

function keysHintOf(question: StudentQuestion, besideShared: boolean): KeysHint {
  const picks =
    questionKind(question) === "choice" && (question.options ?? []).length > 0;
  if (!picks) return "move";
  return besideShared ? "short" : "long";
}

function gapLanding(
  gap: MaterialGap,
  questions: StudentQuestion[],
  numbers: ReadonlyMap<string, number>,
): { index: number; target: string } | null {
  const number = numbers.get(gap.questionId);
  if (number === undefined) return null;
  const blank =
    gap.kind === "blank"
      ? questions[number - 1]?.blanks?.find((item) => item.gapId === gap.blankGapId)
      : undefined;
  return {
    index: number - 1,
    target: blank ? `answer-blank-${blank.id}` : `answer-question-${gap.questionId}`,
  };
}

function useLanding(
  visit: string,
  sheetRef: RefObject<HTMLDivElement | null>,
  panelRef: RefObject<HTMLDivElement | null>,
) {
  const target = useRef<string | null>(null);
  const [request, ask] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    const id = target.current;
    target.current = null;
    if (id === null) {
      if (sheetRef.current) sheetRef.current.scrollTop = 0;
      panelRef.current?.focus({ preventScroll: true });
      return;
    }
    const element = document.getElementById(id) ?? panelRef.current;
    element?.focus({ preventScroll: true });
    element?.scrollIntoView?.({ block: "nearest" });
  }, [visit, request, sheetRef, panelRef]);
  return useCallback((id: string) => {
    target.current = id;
    ask();
  }, []);
}
