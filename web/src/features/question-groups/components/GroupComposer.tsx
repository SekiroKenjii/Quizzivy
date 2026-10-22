import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Editor } from "@tiptap/react";
import { ArrowDown, ArrowUp, FileText, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Checkbox } from "@/components/ui/checkbox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState } from "@/components/shared/ListState";
import { ContentEditor } from "@/components/shared/content/editor/ContentEditor";
import { removeGap } from "@/components/shared/content/editor/gapCommands";
import { isQuestionContent } from "@/components/shared/content/questionContent";
import { QuestionEditor } from "@/features/question-bank/components/QuestionEditor";
import { AudioPolicyPanel } from "@/features/question-bank/components/AudioPolicyPanel";
import { DEFAULT_AUDIO_POLICY } from "@/features/question-bank/audioPolicy";
import type { MediaAsset } from "@/features/media/api";
import type { GroupBundle } from "../api";
import {
  emptyMaterial,
  hasOptionOrderChoice,
  materialAssets,
  memberOptionOrder,
  memberValues,
  newGroupQuestion,
  withMembers,
  type GroupMaterial,
  type GroupRecording,
} from "../model";
import { GapBindings } from "./GapBindings";
import { MaterialContent } from "./MaterialContent";
import { cn } from "@/lib/utils";

/**
 * GroupComposer edits a question group's bundle: its title, instructions,
 * shared materials and member questions, handing every change to `onChange`.
 * `titleFocus` is a request: each new value puts focus in the title field
 * with its text selected, and undefined asks for nothing. A member's question
 * settings sit in a column beside the editor from 1024px and in their dialog
 * below it; `settingsInDialog` keeps them in the dialog at every width.
 */
export function GroupComposer({
  bundle,
  assets,
  onChange,
  onAsset,
  onRefresh,
  selectedQuestionId,
  onSelectedQuestionChange,
  embedded = false,
  titleFocus,
  settingsInDialog = false,
}: Readonly<{
  bundle: GroupBundle;
  assets: MediaAsset[];
  onChange: (bundle: GroupBundle) => void;
  onAsset: (asset: MediaAsset) => void;
  onRefresh: () => void;
  selectedQuestionId?: string | undefined;
  onSelectedQuestionChange?: (id: string | null) => void;
  embedded?: boolean;
  titleFocus?: number | undefined;
  settingsInDialog?: boolean;
}>) {
  const { t } = useTranslation();
  const titleField = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (titleFocus === undefined) return;
    titleField.current?.focus();
    titleField.current?.select();
  }, [titleFocus]);
  const [selected, setSelected] = useState(
    selectedQuestionId ?? bundle.group.stimuli[0]?.id ?? "instructions",
  );
  const [requestedQuestion, setRequestedQuestion] = useState(selectedQuestionId);
  if (requestedQuestion !== selectedQuestionId) {
    setRequestedQuestion(selectedQuestionId);
    setSelected(selectedQuestionId ?? "instructions");
  }
  const [removing, setRemoving] = useState<{
    kind: "material" | "question";
    id: string;
  } | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [starters, setStarters] = useState<ReadonlySet<string>>(new Set());
  const recordings = useRef(new Map<string, GroupRecording>());
  const assetById = useMemo(
    () => new Map(assets.map((asset) => [asset.id, asset])),
    [assets],
  );
  const questions = new Map(
    bundle.questions.map((question) => [question.id, question]),
  );
  const member = questions.get(selected);
  const material = bundle.group.stimuli.find((item) => item.id === selected);
  const bound = new Set(
    bundle.group.stimuli.flatMap((item) =>
      item.gaps.map((binding) => binding.questionId),
    ),
  );

  function selectItem(id: string, newQuestion = false) {
    setSelected(id);
    if (onSelectedQuestionChange) {
      const questionId = newQuestion || questions.has(id) ? id : undefined;
      setRequestedQuestion(questionId);
      onSelectedQuestionChange(questionId ?? null);
    }
  }

  function changeMaterials(stimuli: GroupMaterial[]) {
    for (const recording of bundle.group.recordings)
      recordings.current.set(recording.assetId, recording);
    const shared = [...materialAssets(stimuli)].flatMap(([assetId, kind]) => {
      if (kind !== "audio") return [];
      const existing = recordings.current.get(assetId);
      const recording = existing ?? {
        id: crypto.randomUUID(),
        assetId,
        policy: { ...DEFAULT_AUDIO_POLICY },
        transcript: null,
      };
      recordings.current.set(assetId, recording);
      return [recording];
    });
    onChange({ ...bundle, group: { ...bundle.group, stimuli, recordings: shared } });
  }
  function changeMaterial(next: GroupMaterial) {
    changeMaterials(
      bundle.group.stimuli.map((item) => (item.id === next.id ? next : item)),
    );
  }
  function addQuestion() {
    const question = newGroupQuestion(t);
    onChange(withMembers(bundle, [question]));
    setStarters((current) => new Set([...current, question.id]));
    selectItem(question.id, true);
  }
  function addMaterial() {
    const item = emptyMaterial(
      t("groups.newMaterial", { number: bundle.group.stimuli.length + 1 }),
    );
    changeMaterials([...bundle.group.stimuli, item]);
    selectItem(item.id);
  }
  function moveMaterial(id: string, direction: -1 | 1) {
    const stimuli = [...bundle.group.stimuli];
    const index = stimuli.findIndex((item) => item.id === id);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= stimuli.length) return;
    [stimuli[index], stimuli[target]] = [stimuli[target]!, stimuli[index]!];
    changeMaterials(stimuli);
  }
  function moveMember(index: number, direction: -1 | 1) {
    const members = [...bundle.group.members];
    const target = index + direction;
    if (target < 0 || target >= members.length) return;
    [members[index], members[target]] = [members[target]!, members[index]!];
    onChange({ ...bundle, group: { ...bundle.group, members } });
  }
  function remove() {
    if (!removing) return;
    if (removing.kind === "material")
      changeMaterials(bundle.group.stimuli.filter((item) => item.id !== removing.id));
    else if (!bound.has(removing.id))
      onChange({
        ...bundle,
        questions: bundle.questions.filter((item) => item.id !== removing.id),
        group: {
          ...bundle.group,
          members: bundle.group.members.filter(
            (item) => item.questionId !== removing.id,
          ),
        },
      });
    selectItem("instructions");
    setRemoving(null);
  }

  return (
    <div className="@container/composer flex min-w-0 flex-col gap-5">
      <Field>
        <FieldLabel htmlFor="shared-group-title">{t("groups.titleLabel")}</FieldLabel>
        <Input
          ref={titleField}
          id="shared-group-title"
          value={bundle.group.title}
          maxLength={200}
          onChange={(event) =>
            onChange({
              ...bundle,
              group: { ...bundle.group, title: event.target.value },
            })
          }
        />
        <FieldDescription>{t("groups.independentHint")}</FieldDescription>
      </Field>
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-5 @min-[720px]/composer:grid-cols-[12rem_minmax(0,1fr)]">
        <CompactGroupNavigation
          enabled={embedded}
          bundle={bundle}
          selected={selected}
          onSelect={(id) => selectItem(id)}
          onAddMaterial={addMaterial}
          onAddQuestion={addQuestion}
        />
        <nav
          aria-label={t("groups.contents")}
          className={cn(
            "flex flex-col gap-3",
            embedded && "hidden @min-[720px]/composer:flex",
          )}
        >
          <Button
            variant={selected === "instructions" ? "secondary" : "ghost"}
            className="justify-start"
            onClick={() => selectItem("instructions")}
          >
            <FileText aria-hidden="true" />
            {t("groups.instructions")}
          </Button>
          <p className="text-muted-foreground text-xs font-medium">
            {t("groups.materials")}
          </p>
          {bundle.group.stimuli.map((item) => (
            <Button
              key={item.id}
              variant={selected === item.id ? "secondary" : "ghost"}
              className="h-auto justify-start text-left whitespace-normal"
              onClick={() => selectItem(item.id)}
            >
              {item.title || t("groups.materials")}
            </Button>
          ))}
          <Button
            variant="outline"
            size="sm"
            disabled={bundle.group.stimuli.length >= 16}
            onClick={addMaterial}
          >
            <Plus aria-hidden="true" />
            {t("groups.addMaterial")}
          </Button>
          <Separator />
          <p className="text-muted-foreground text-xs font-medium">
            {t("groups.questions", { count: bundle.group.members.length })}
          </p>
          <ol className="flex flex-col gap-1">
            {bundle.group.members.map((item, index) => (
              <li key={item.questionId}>
                <Button
                  variant={selected === item.questionId ? "secondary" : "ghost"}
                  className="h-auto w-full justify-start text-left whitespace-normal"
                  onClick={() => selectItem(item.questionId)}
                >
                  <span className="shrink-0 tabular-nums">{index + 1}</span>
                  <span className="line-clamp-2">
                    {questions.get(item.questionId)?.input.prompt ||
                      t("builder.untitledQuestion")}
                  </span>
                </Button>
              </li>
            ))}
          </ol>
          <Button
            variant="outline"
            size="sm"
            disabled={bundle.group.members.length >= 200}
            onClick={addQuestion}
          >
            <Plus aria-hidden="true" />
            {t("builder.addQuestion")}
          </Button>
        </nav>
        <div className="flex min-w-0 flex-col gap-5">
          {selected === "instructions" ? (
            <FieldGroup>
              <Field>
                <FieldLabel>{t("groups.instructions")}</FieldLabel>
                <FieldDescription>{t("groups.instructionsHint")}</FieldDescription>
                <ContentEditor
                  key="instructions"
                  initialContent={
                    bundle.group.instructions ?? {
                      format: "semantic_v1",
                      blocks: [{ type: "paragraph", content: [] }],
                    }
                  }
                  label={t("groups.instructions")}
                  profile="question"
                  onChange={(content) => {
                    if (isQuestionContent(content))
                      onChange({
                        ...bundle,
                        group: { ...bundle.group, instructions: content },
                      });
                  }}
                />
              </Field>
            </FieldGroup>
          ) : null}
          {material ? (
            <MaterialPane
              key={material.id}
              material={material}
              bundle={bundle}
              assets={assetById}
              onAsset={onAsset}
              onRefresh={onRefresh}
              onChange={changeMaterial}
              onMove={(direction) => moveMaterial(material.id, direction)}
              first={bundle.group.stimuli[0]?.id === material.id}
              last={bundle.group.stimuli.at(-1)?.id === material.id}
              onRecording={(recording) =>
                onChange({
                  ...bundle,
                  group: {
                    ...bundle.group,
                    recordings: bundle.group.recordings.map((item) =>
                      item.id === recording.id ? recording : item,
                    ),
                  },
                })
              }
              onRemove={() => setRemoving({ kind: "material", id: material.id })}
            />
          ) : null}
          {member ? (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label={t("groups.moveMemberUp")}
                  disabled={bundle.group.members[0]?.questionId === member.id}
                  onClick={() =>
                    moveMember(
                      bundle.group.members.findIndex(
                        (item) => item.questionId === member.id,
                      ),
                      -1,
                    )
                  }
                >
                  <ArrowUp aria-hidden="true" />
                </Button>
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label={t("groups.moveMemberDown")}
                  disabled={bundle.group.members.at(-1)?.questionId === member.id}
                  onClick={() =>
                    moveMember(
                      bundle.group.members.findIndex(
                        (item) => item.questionId === member.id,
                      ),
                      1,
                    )
                  }
                >
                  <ArrowDown aria-hidden="true" />
                </Button>
                <SettingsButton
                  always={settingsInDialog}
                  onOpen={() => setSettingsOpen(true)}
                />
                <Button
                  variant="ghost"
                  size="sm"
                  className="ml-auto"
                  disabled={bound.has(member.id)}
                  title={bound.has(member.id) ? t("groups.removeBoundHint") : undefined}
                  onClick={() => setRemoving({ kind: "question", id: member.id })}
                >
                  <Trash2 aria-hidden="true" />
                  {t("groups.removeQuestion")}
                </Button>
              </div>
              {bound.has(member.id) ? (
                <Alert>
                  <AlertDescription>{t("groups.removeBoundHint")}</AlertDescription>
                </Alert>
              ) : null}
              <QuestionEditor
                key={member.id}
                value={memberValues(member.input)}
                asset={assetById.get(member.input.mediaAssetId ?? "") ?? null}
                onAssetChange={(asset) => {
                  if (asset) onAsset(asset);
                }}
                assets={assetById}
                onAsset={onAsset}
                onRefresh={onRefresh}
                clearPromptOnFocus={starters.has(member.id)}
                settings={{
                  hideBelow: "lg",
                  open: settingsOpen,
                  onOpenChange: setSettingsOpen,
                  always: settingsInDialog,
                }}
                contextLabel={bundle.group.title}
                onChange={(input) =>
                  onChange({
                    ...bundle,
                    questions: bundle.questions.map((item) =>
                      item.id === member.id ? { ...item, input } : item,
                    ),
                    group: {
                      ...bundle.group,
                      members: bundle.group.members.map((item) =>
                        item.questionId === member.id
                          ? {
                              ...item,
                              optionOrder: memberOptionOrder(
                                input.type,
                                item.optionOrder,
                                member.input.type,
                              ),
                            }
                          : item,
                      ),
                    },
                  })
                }
              />
              {hasOptionOrderChoice(member.input.type) ? (
                <Field orientation="horizontal">
                  <Checkbox
                    id={`group-fixed-${member.id}`}
                    checked={
                      bundle.group.members.find((item) => item.questionId === member.id)
                        ?.optionOrder === "fixed"
                    }
                    onChange={(event) =>
                      onChange({
                        ...bundle,
                        group: {
                          ...bundle.group,
                          members: bundle.group.members.map((item) =>
                            item.questionId === member.id
                              ? {
                                  ...item,
                                  optionOrder: event.target.checked
                                    ? "fixed"
                                    : "shuffle",
                                }
                              : item,
                          ),
                        },
                      })
                    }
                  />
                  <FieldLabel htmlFor={`group-fixed-${member.id}`}>
                    {t("groups.fixedOptions")}
                  </FieldLabel>
                </Field>
              ) : null}
            </>
          ) : null}
          {!member && !material && selected !== "instructions" ? (
            <EmptyState>{t("groups.selectContent")}</EmptyState>
          ) : null}
        </div>
      </div>
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={t("groups.removeTitle")}
        description={t(
          removing?.kind === "material"
            ? "groups.removeMaterialBody"
            : "groups.removeQuestionBody",
        )}
        confirmLabel={t("common.delete")}
        destructive
        onConfirm={remove}
      />
    </div>
  );
}

function SettingsButton({
  always,
  onOpen,
}: Readonly<{ always: boolean; onOpen: () => void }>) {
  const { t } = useTranslation();
  return (
    <Button
      variant="outline"
      size="sm"
      className={always ? undefined : "lg:hidden"}
      onClick={onOpen}
    >
      {t("questionEditor.settings")}
    </Button>
  );
}

function MaterialPane({
  material,
  bundle,
  assets,
  onAsset,
  onRefresh,
  onChange,
  onRecording,
  onRemove,
  onMove,
  first,
  last,
}: Readonly<{
  material: GroupMaterial;
  bundle: GroupBundle;
  assets: ReadonlyMap<string, MediaAsset>;
  onAsset: (asset: MediaAsset) => void;
  onRefresh: () => void;
  onChange: (material: GroupMaterial) => void;
  onRecording: (recording: GroupRecording) => void;
  onRemove: () => void;
  onMove: (direction: -1 | 1) => void;
  first: boolean;
  last: boolean;
}>) {
  const { t } = useTranslation();
  const [preview, setPreview] = useState(false);
  const [editor, setEditor] = useState<Editor | null>(null);
  return (
    <div className="flex min-w-0 flex-col gap-5">
      <Field>
        <FieldLabel htmlFor={`material-title-${material.id}`}>
          {t("groups.materialTitle")}
        </FieldLabel>
        <Input
          id={`material-title-${material.id}`}
          value={material.title}
          maxLength={200}
          onChange={(event) => onChange({ ...material, title: event.target.value })}
        />
      </Field>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="icon-sm"
          aria-label={t("groups.moveMaterialUp")}
          disabled={first}
          onClick={() => onMove(-1)}
        >
          <ArrowUp aria-hidden="true" />
        </Button>
        <Button
          variant="outline"
          size="icon-sm"
          aria-label={t("groups.moveMaterialDown")}
          disabled={last}
          onClick={() => onMove(1)}
        >
          <ArrowDown aria-hidden="true" />
        </Button>
        <Button variant="outline" size="sm" onClick={() => setPreview(!preview)}>
          {t(preview ? "groups.editMaterial" : "groups.previewMaterial")}
        </Button>
        <Button variant="ghost" size="sm" className="ml-auto" onClick={onRemove}>
          <Trash2 aria-hidden="true" />
          {t("groups.removeMaterial")}
        </Button>
      </div>
      <MaterialContent
        material={material}
        preview={preview}
        assets={assets}
        onChange={onChange}
        onAsset={onAsset}
        onRefresh={onRefresh}
        onEditor={setEditor}
      />
      <GapBindings
        material={material}
        bundle={bundle}
        onChange={onChange}
        onRemove={editor ? (gapId) => removeGap(editor, gapId) : undefined}
      />
      <RecordingSettings
        recordings={bundle.group.recordings.filter(
          (recording) => materialAssets([material]).get(recording.assetId) === "audio",
        )}
        assets={assets}
        onChange={onRecording}
      />
    </div>
  );
}

function RecordingSettings({
  recordings,
  assets,
  onChange,
}: Readonly<{
  recordings: GroupRecording[];
  assets: ReadonlyMap<string, MediaAsset>;
  onChange: (recording: GroupRecording) => void;
}>) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState("");
  const recording = recordings.find((item) => item.id === selected) ?? recordings[0];
  if (!recording) return null;
  return (
    <FieldGroup>
      <Field>
        <FieldLabel htmlFor="group-recording-policy">
          {t("groups.recordingPolicy")}
        </FieldLabel>
        <Select value={recording.id} onValueChange={setSelected}>
          <SelectTrigger id="group-recording-policy">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {recordings.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  {assets.get(item.assetId)?.originalFilename ?? t("groups.audio")}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        <FieldDescription>{t("groups.sharedPolicyHint")}</FieldDescription>
      </Field>
      <AudioPolicyPanel
        key={recording.id}
        policy={recording.policy}
        transcript={recording.transcript ?? ""}
        onPolicyChange={(policy) => onChange({ ...recording, policy })}
        onTranscriptChange={(transcript) => onChange({ ...recording, transcript })}
      />
    </FieldGroup>
  );
}

function CompactGroupNavigation({
  enabled,
  bundle,
  selected,
  onSelect,
  onAddMaterial,
  onAddQuestion,
}: Readonly<{
  enabled: boolean;
  bundle: GroupBundle;
  selected: string;
  onSelect: (id: string) => void;
  onAddMaterial: () => void;
  onAddQuestion: () => void;
}>) {
  const { t } = useTranslation();
  const questions = new Map(
    bundle.questions.map((question) => [question.id, question]),
  );
  if (!enabled) return null;
  return (
    <div className="flex flex-col gap-3 @min-[720px]/composer:hidden">
      <Field>
        <FieldLabel htmlFor="group-content-picker">{t("groups.contents")}</FieldLabel>
        <Select value={selected} onValueChange={onSelect}>
          <SelectTrigger id="group-content-picker">
            <SelectValue />
          </SelectTrigger>
          <SelectContent className="max-w-(--radix-select-trigger-width)">
            <SelectGroup>
              <SelectItem value="instructions">{t("groups.instructions")}</SelectItem>
              {bundle.group.stimuli.map((item) => (
                <SelectItem
                  key={item.id}
                  value={item.id}
                  className="*:[span]:last:min-w-0"
                >
                  <span className="truncate">
                    {item.title || t("groups.materials")}
                  </span>
                </SelectItem>
              ))}
              {bundle.group.members.map((item, index) => (
                <SelectItem
                  key={item.questionId}
                  value={item.questionId}
                  className="*:[span]:last:min-w-0"
                >
                  <span className="truncate">
                    {index + 1}{" "}
                    {questions.get(item.questionId)?.input.prompt ||
                      t("builder.untitledQuestion")}
                  </span>
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={bundle.group.stimuli.length >= 16}
          onClick={onAddMaterial}
        >
          <Plus />
          {t("groups.addMaterial")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={bundle.group.members.length >= 200}
          onClick={onAddQuestion}
        >
          <Plus />
          {t("builder.addQuestion")}
        </Button>
      </div>
    </div>
  );
}
