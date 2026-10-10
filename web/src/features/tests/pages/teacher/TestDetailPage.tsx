import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  ClipboardList,
  FileClock,
  GitCompare,
  History,
  Monitor,
  Pencil,
  PencilLine,
  Smartphone,
} from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { LoadError } from "@/components/shared/ListState";
import { Sheet } from "@/components/shared/Sheet";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import {
  createDraftFromTestVersion,
  deleteTestVersion,
  getTest,
  getVersionDiff,
  listVersions,
  previewTest,
  publishTest,
  setCurrentTestVersion,
  type Test,
  type TestStatus,
  type TestVersion,
} from "@/features/tests/api";
import {
  StudentPreview,
  type PreviewMark,
} from "@/features/tests/components/StudentPreview";
import { VersionChanges } from "@/features/tests/components/VersionChanges";
import {
  VersionHistory,
  type VersionAction,
} from "@/features/tests/components/VersionHistory";
import { assignedLabel } from "@/features/tests/testFacts";
import { diffSummary, previewMarks } from "@/features/tests/versionDiff";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";

const TWO_COLUMNS = 1000;

const PILL_SHAPE =
  "border-0 in-data-[scale=deck]:rounded-full in-data-[scale=deck]:px-2";

const PILL: Record<TestStatus, string> = {
  published: `${PILL_SHAPE} [&>[aria-hidden]]:bg-success`,
  draft: `${PILL_SHAPE} bg-muted text-muted-fg [&>[aria-hidden]]:bg-muted-fg`,
  archived: `${PILL_SHAPE} bg-muted text-muted-fg [&>[aria-hidden]]:bg-muted-fg`,
};

const HEAD_BUTTON = "shadow-card h-9";

const BANNER_BUTTON =
  "h-8 rounded-md px-3 text-sm in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm";

type PublishValues = { note: string };

const PUBLISH_INITIAL: PublishValues = { note: "" };

type Params = Readonly<{
  version: number | undefined;
  compare: boolean;
  phone: boolean;
  history: boolean;
}>;

type ParamChange = Partial<
  Record<"version" | "compare" | "device" | "history", string | null>
>;

function positive(value: string | null): number | undefined {
  const n = Number(value);
  return Number.isSafeInteger(n) && n > 0 ? n : undefined;
}

function useDetailParams(): [Params, (change: ParamChange) => void] {
  const location = useLocation();
  const navigate = useNavigate();
  const search = new URLSearchParams(location.search);
  const params: Params = {
    version: positive(search.get("version")),
    compare: search.get("compare") === "1",
    phone: search.get("device") === "phone",
    history: search.get("history") === "1" || location.hash === "#versions",
  };
  const update = (change: ParamChange) => {
    const next = new URLSearchParams(location.search);
    for (const [key, value] of Object.entries(change)) {
      if (typeof value === "string") next.set(key, value);
      else next.delete(key);
    }
    const query = next.toString();
    void navigate(
      { pathname: location.pathname, search: query ? `?${query}` : "", hash: "" },
      { replace: true },
    );
  };
  return [params, update];
}

function previousOf(
  items: readonly TestVersion[],
  version: number,
): number | undefined {
  let previous: number | undefined;
  for (const item of items) {
    if (item.version < version && (previous === undefined || item.version > previous))
      previous = item.version;
  }
  return previous;
}

function diffOptions(id: string, version: number, previous: number) {
  return {
    queryKey: ["admin-test-diff", id, version, previous],
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      getVersionDiff(id, version, previous, signal),
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  };
}

/**
 * TestDetailPage is a test's detail in the teacher workspace: what a student
 * receives from the version chosen, what changed from the version before it,
 * and the history of every version, with the draft's unpublished changes
 * offered for publishing. `?version=` picks the version shown, `compare=1`
 * lists its changes, `device=phone` draws the phone frame and `history=1`
 * (or the builder's `#versions`) opens the history sheet below the two
 * columns' width.
 */
export default function TestDetailPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const test = useQuery({
    queryKey: ["admin-test", id],
    queryFn: ({ signal }) => getTest(id, signal),
  });
  if (test.isPending) return <DetailSkeleton />;
  if (test.isError)
    return (
      <LoadError error={test.error} onRetry={() => void test.refetch()}>
        {t("tests.detail.loadFailed")}
      </LoadError>
    );
  return <TestDetail test={test.data} />;
}

function DetailSkeleton() {
  const wide = useContentWidthAtLeast(TWO_COLUMNS);
  return (
    <div aria-busy="true" className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-4 w-16" />
        <Skeleton className="h-7.5 w-72 max-w-full" />
        <Skeleton className="h-4 w-80 max-w-full" />
      </div>
      <div
        className={cn(
          "grid items-start gap-4",
          wide ? "grid-cols-[minmax(0,1fr)_340px]" : "grid-cols-1",
        )}
      >
        <Skeleton className="h-96 rounded-xl" />
        {wide ? <Skeleton className="h-72 rounded-xl" /> : null}
      </div>
    </div>
  );
}

function TestDetail({ test }: Readonly<{ test: Test }>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const wide = useContentWidthAtLeast(TWO_COLUMNS);
  const [params, update] = useDetailParams();
  const [publishing, setPublishing] = useState(false);
  const [action, setAction] = useState<{ kind: VersionAction; version: number } | null>(
    null,
  );
  const id = test.id;
  const archived = test.status === "archived";

  const versions = useQuery({
    queryKey: ["admin-test-versions", id],
    queryFn: ({ signal }) => listVersions(id, signal),
  });
  const items = versions.data?.items;
  const known =
    params.version !== undefined &&
    (items === undefined || items.some((item) => item.version === params.version));
  const fallback = test.currentVersion > 0 ? test.currentVersion : undefined;
  const selected = known ? params.version : fallback;
  const previous =
    items !== undefined && selected !== undefined
      ? previousOf(items, selected)
      : undefined;
  const summaries = useVersionSummaries(id, items ?? []);

  const refresh = (deleted?: number) =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ["admin-test", id] }),
      queryClient.invalidateQueries({ queryKey: ["admin-test-versions", id] }),
      queryClient.invalidateQueries({
        queryKey: ["admin-test-preview", id],
        predicate: (query) => deleted === undefined || query.queryKey[2] !== deleted,
      }),
      queryClient.invalidateQueries({ queryKey: ["admin-tests"] }),
      queryClient.invalidateQueries({ queryKey: ["admin-questions"] }),
    ]);

  const change = useMutation({
    mutationFn: async (input: { kind: VersionAction; version: number }) => {
      if (input.kind === "delete") await deleteTestVersion(id, input.version);
      else if (input.kind === "draft")
        await createDraftFromTestVersion(test, input.version);
      else await setCurrentTestVersion(test, input.version);
    },
    onSuccess: async (_, input) => {
      setAction(null);
      const deleted = input.kind === "delete" ? input.version : undefined;
      if (deleted !== undefined && deleted === selected)
        update({ version: null, compare: null });
      await refresh(deleted);
      if (deleted !== undefined)
        queryClient.removeQueries({
          queryKey: ["admin-test-preview", id, deleted],
          exact: true,
          type: "inactive",
        });
      if (input.kind === "draft") await navigate(`/teacher/tests/${id}/edit`);
      notify.success(t(DONE[input.kind], { n: input.version }));
    },
  });

  const history = (
    <VersionHistory
      test={test}
      query={versions}
      selected={selected}
      summaries={summaries}
      pending={change.isPending}
      onSelect={(version) =>
        update({ version: String(version), compare: null, history: null })
      }
      onAction={(kind, version) => {
        change.reset();
        setAction({ kind, version });
      }}
    />
  );

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <DetailHead test={test} wide={wide} onHistory={() => update({ history: "1" })} />
      {(test.unpublishedChanges ?? 0) > 0 && !archived ? (
        <DraftBanner
          test={test}
          next={test.nextVersion}
          onPublish={() => setPublishing(true)}
        />
      ) : null}
      <div
        className={cn(
          "grid items-start gap-4",
          wide ? "grid-cols-[minmax(0,1fr)_340px]" : "grid-cols-1",
        )}
      >
        <PreviewPanel
          test={test}
          selected={selected}
          previous={previous}
          params={params}
          update={update}
        />
        {wide ? (
          <aside
            id="versions"
            aria-labelledby="test-version-history"
            className="bg-card shadow-card sticky top-0 flex min-w-0 flex-col gap-2.5 rounded-xl border p-3.5"
          >
            <h2 id="test-version-history" className="text-base font-semibold">
              {t("tests.detail.versionHistory")}
            </h2>
            {history}
          </aside>
        ) : null}
      </div>
      <Sheet
        open={!wide && params.history}
        onOpenChange={(open) => {
          if (!open) update({ history: null });
        }}
        title={t("tests.detail.versionHistory")}
        width={380}
      >
        {history}
      </Sheet>
      <PublishDialog
        test={test}
        open={publishing}
        next={test.nextVersion}
        kept={
          items?.find((item) => item.version === test.currentVersion)
            ?.assignmentCount ?? 0
        }
        onOpenChange={setPublishing}
        onPublished={async () => {
          setPublishing(false);
          update({ version: null, compare: null });
          await refresh();
        }}
      />
      <VersionDialog
        action={action}
        draftAhead={(test.unpublishedChanges ?? 0) > 0}
        pending={change.isPending}
        error={change.error}
        onCancel={() => {
          if (!change.isPending) setAction(null);
        }}
        onConfirm={(input) => change.mutate(input)}
      />
    </div>
  );
}

const DONE: Record<VersionAction, string> = {
  draft: "tests.detail.restore.done",
  current: "tests.detail.makeDefault.done",
  delete: "tests.detail.deleteVersion.done",
};

function useVersionSummaries(
  id: string,
  items: readonly TestVersion[],
): ReadonlyMap<number, string> {
  const { t } = useTranslation();
  const targets = items
    .filter((item) => !item.changeNote)
    .map((item) => ({
      version: item.version,
      previous: previousOf(items, item.version),
    }));
  const compared = targets.filter(
    (target): target is { version: number; previous: number } =>
      target.previous !== undefined,
  );
  const diffs = useQueries({
    queries: compared.map((target) => diffOptions(id, target.version, target.previous)),
  });
  const summaries = new Map<number, string>();
  for (const target of targets) {
    if (target.previous === undefined && target.version === 1)
      summaries.set(target.version, t("tests.detail.history.firstVersion"));
  }
  compared.forEach((target, index) => {
    const data = diffs[index]?.data;
    if (data) summaries.set(target.version, diffSummary(data.changes, t));
  });
  return summaries;
}

function DetailHead({
  test,
  wide,
  onHistory,
}: Readonly<{ test: Test; wide: boolean; onHistory: () => void }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const assignLocked = assignLockedReason(test, t);
  return (
    <PageHead
      title={nfc(test.title)}
      crumb
      back={{ to: "/teacher/tests", label: t("teacherShell.nav.tests") }}
      status={
        <StatusBadge
          kind="test"
          status={test.status}
          dot
          className={PILL[test.status]}
        />
      }
      actions={
        <>
          {wide ? null : (
            <Button variant="outline" className={HEAD_BUTTON} onClick={onHistory}>
              <History aria-hidden="true" />
              {t("tests.detail.versionHistory")}
            </Button>
          )}
          <Button asChild variant="outline" className={HEAD_BUTTON}>
            <Link to={`/teacher/tests/${test.id}/edit`}>
              <Pencil aria-hidden="true" />
              {t("tests.detail.openBuilder")}
            </Link>
          </Button>
          {assignLocked === null ? (
            <Button asChild className="h-9">
              <Link to={`/teacher/assignments/new?test=${test.id}`}>
                <ClipboardList aria-hidden="true" />
                {t("tests.detail.assign")}
              </Link>
            </Button>
          ) : (
            <Button
              className="aria-disabled:hover:bg-primary h-9 aria-disabled:cursor-default aria-disabled:opacity-50"
              aria-disabled="true"
              title={assignLocked}
            >
              <ClipboardList aria-hidden="true" />
              {t("tests.detail.assign")}
            </Button>
          )}
        </>
      }
    >
      <p className="text-muted-fg text-sm">
        {t("tests.detail.meta", {
          questions: t("tests.questionCount", { count: test.questionCount }),
          points: t("tests.pointCount", {
            count: test.totalPoints,
            points: new Intl.NumberFormat(locale).format(test.totalPoints),
          }),
          assigned: assignedLabel(test, t),
        })}
      </p>
    </PageHead>
  );
}

function assignLockedReason(
  test: Test,
  t: ReturnType<typeof useTranslation>["t"],
): string | null {
  if (test.status === "archived") return t("tests.detail.history.restoreLocked");
  if (test.currentVersion === 0) return t("tests.detail.assignLocked");
  return null;
}

function DraftBanner({
  test,
  next,
  onPublish,
}: Readonly<{ test: Test; next: number; onPublish: () => void }>) {
  const { t } = useTranslation();
  return (
    <div className="bg-warning-soft text-warning-ink flex flex-wrap items-center gap-3 rounded-xl px-3.5 py-3">
      <PencilLine aria-hidden="true" className="size-4.25 flex-none" />
      <p className="text-ui text-fg min-w-0 flex-[1_1_280px] leading-normal">
        <b className="font-semibold">{t("tests.detail.banner.title")}</b>{" "}
        {t("tests.detail.banner.body", { current: test.currentVersion, next })}
      </p>
      <Button
        asChild
        variant="outline"
        className={cn(BANNER_BUTTON, "text-fg shadow-none")}
      >
        <Link to={`/teacher/tests/${test.id}/edit`}>
          {t("tests.detail.banner.review")}
        </Link>
      </Button>
      <Button className={cn(BANNER_BUTTON, "font-semibold")} onClick={onPublish}>
        {t("tests.detail.banner.publish", { n: next })}
      </Button>
    </div>
  );
}

function PreviewPanel({
  test,
  selected,
  previous,
  params,
  update,
}: Readonly<{
  test: Test;
  selected: number | undefined;
  previous: number | undefined;
  params: Params;
  update: (change: ParamChange) => void;
}>) {
  const { t } = useTranslation();
  const headingId = useId();
  const id = test.id;
  const preview = useQuery({
    queryKey: ["admin-test-preview", id, selected],
    queryFn: ({ signal }) => previewTest(id, selected, signal),
    enabled: selected !== undefined,
    retry: false,
  });
  const compareOn = params.compare && previous !== undefined;
  const diff = useQuery({
    ...diffOptions(id, selected ?? 0, previous ?? 0),
    enabled: compareOn && selected !== undefined,
  });
  const notPublished =
    selected === undefined ||
    (preview.error instanceof ApiError && preview.error.code === "TEST_NOT_PUBLISHED");
  return (
    <section
      aria-labelledby={headingId}
      className="bg-card shadow-card min-w-0 overflow-hidden rounded-xl border"
    >
      <div className="flex flex-wrap items-center justify-between gap-2.5 border-b px-4 py-3">
        <div className="min-w-0">
          <h2 id={headingId} className="text-md font-semibold tracking-[-0.01em]">
            {t("tests.detail.preview.title")}
          </h2>
          <p className="text-muted-fg text-meta mt-0.5">
            {previewOf(test, selected, notPublished, t)}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {previous !== undefined && !notPublished ? (
            <button
              type="button"
              aria-pressed={compareOn}
              onClick={() => update({ compare: compareOn ? null : "1" })}
              className={cn(
                "text-meta inline-flex h-8.5 cursor-pointer items-center gap-1.5 rounded-md border px-2.75 font-medium whitespace-nowrap",
                compareOn
                  ? "bg-primary text-primary-fg border-primary"
                  : "bg-card text-fg hover:bg-muted",
              )}
            >
              <GitCompare aria-hidden="true" className="size-3.5 flex-none" />
              {t("tests.detail.compare.toggle", { n: previous })}
            </button>
          ) : null}
          <Segmented
            size="sm"
            label={t("tests.detail.preview.device")}
            value={params.phone ? "phone" : "desktop"}
            onChange={(value) => update({ device: value === "phone" ? "phone" : null })}
            options={[
              {
                value: "desktop",
                label: t("tests.detail.preview.computer"),
                icon: Monitor,
              },
              {
                value: "phone",
                label: t("tests.detail.preview.phone"),
                icon: Smartphone,
              },
            ]}
          />
        </div>
      </div>
      {compareOn && !notPublished ? (
        <VersionChanges query={diff} previous={previous} />
      ) : null}
      {notPublished ? (
        <NotPublished testId={id} />
      ) : (
        <PreviewFrame
          preview={preview}
          phone={params.phone}
          marks={
            compareOn && diff.data ? previewMarks(diff.data.changes, t) : undefined
          }
        />
      )}
    </section>
  );
}

function previewOf(
  test: Test,
  selected: number | undefined,
  notPublished: boolean,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  if (notPublished || selected === undefined) return t("tests.detail.preview.nothing");
  return selected === test.currentVersion
    ? t("tests.detail.preview.current", { n: selected })
    : t("tests.detail.preview.older", { n: selected });
}

function NotPublished({ testId }: Readonly<{ testId: string }>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col items-center gap-3 px-5 py-10 text-center">
      <span className="bg-muted grid size-11 place-items-center rounded-xl">
        <FileClock aria-hidden="true" className="size-5" />
      </span>
      <p className="text-muted-fg max-w-[420px] text-base leading-[1.55]">
        {t("tests.detail.preview.notPublished")}
      </p>
      <Button asChild className="h-9">
        <Link to={`/teacher/tests/${testId}/edit`}>
          {t("tests.detail.openBuilder")}
        </Link>
      </Button>
    </div>
  );
}

function PreviewFrame({
  preview,
  phone,
  marks,
}: Readonly<{
  preview: UseQueryResult<Awaited<ReturnType<typeof previewTest>>>;
  phone: boolean;
  marks: ReadonlyMap<string, PreviewMark> | undefined;
}>) {
  const { t } = useTranslation();
  return (
    <div className="bg-sidebar max-h-[680px] overflow-y-auto p-5">
      <div
        data-preview-viewport={phone ? "phone" : "desktop"}
        className={cn(
          "bg-bg mx-auto flex max-w-full min-w-0 flex-col gap-4.5 border p-5.5",
          phone ? "w-[390px] rounded-[24px]" : "w-full rounded-xl",
        )}
      >
        {preview.isPending ? (
          <div aria-busy="true" className="flex flex-col gap-3">
            <Skeleton className="h-5 w-1/2" />
            <Skeleton className="h-32 w-full rounded-xl" />
            <Skeleton className="h-32 w-full rounded-xl" />
          </div>
        ) : null}
        {preview.isError ? (
          <LoadError error={preview.error} onRetry={() => void preview.refetch()}>
            {t("tests.detail.preview.failed")}
          </LoadError>
        ) : null}
        {preview.isSuccess ? (
          <>
            <StudentPreview
              questions={preview.data.questions}
              sections={preview.data.sections ?? []}
              groups={preview.data.groups ?? []}
              marks={marks}
              onRetryMedia={() => void preview.refetch()}
            />
            <p className="text-muted-fg text-meta text-center">
              {t("tests.detail.preview.foot", { count: preview.data.questions.length })}
            </p>
          </>
        ) : null}
      </div>
    </div>
  );
}

function PublishDialog({
  test,
  open,
  next,
  kept,
  onOpenChange,
  onPublished,
}: Readonly<{
  test: Test;
  open: boolean;
  next: number;
  kept: number;
  onOpenChange: (open: boolean) => void;
  onPublished: () => Promise<void>;
}>) {
  const { t } = useTranslation();
  const publish = useMutation({
    mutationFn: (note: string) => publishTest(test.id, note),
    onSuccess: async (version) => {
      notify.success(t("tests.detail.publish.done", { n: version.version }));
      await onPublished();
    },
  });
  const fields: readonly FormField<PublishValues>[] = [
    {
      name: "note",
      kind: "area",
      label: t("tests.detail.publish.note"),
      hint: t("tests.detail.publish.noteHint"),
      placeholder: t("tests.detail.publish.notePlaceholder"),
      maxLength: 200,
    },
  ];
  return (
    <FormDialog
      open={open}
      onOpenChange={(value) => {
        if (publish.isPending) return;
        if (value) publish.reset();
        onOpenChange(value);
      }}
      title={t("tests.detail.publish.title", { n: next })}
      description={
        kept > 0
          ? t("tests.detail.publish.bodyKept", {
              count: kept,
              version: test.currentVersion,
            })
          : t("tests.detail.publish.bodyNone")
      }
      initial={PUBLISH_INITIAL}
      fields={fields}
      submitLabel={t("tests.detail.publish.confirm")}
      pending={publish.isPending}
      error={publishError(publish.error, t)}
      onSubmit={(values) => publish.mutate(values.note)}
    />
  );
}

function publishError(
  error: Error | null,
  t: ReturnType<typeof useTranslation>["t"],
): string | null {
  if (error === null) return null;
  if (error instanceof ApiError && error.code === "PUBLISH_VALIDATION_FAILED")
    return t("tests.detail.publish.blocked", { count: error.violations.length });
  return error instanceof ApiError ? error.message : t("tests.detail.publish.failed");
}

const DIALOG: Record<
  VersionAction,
  { title: string; confirm: string; destructive: boolean }
> = {
  draft: {
    title: "tests.detail.restore.title",
    confirm: "tests.detail.restore.confirm",
    destructive: false,
  },
  current: {
    title: "tests.detail.makeDefault.title",
    confirm: "tests.detail.makeDefault.confirm",
    destructive: false,
  },
  delete: {
    title: "tests.detail.deleteVersion.title",
    confirm: "tests.detail.deleteVersion.confirm",
    destructive: true,
  },
};

function dialogBody(kind: VersionAction, draftAhead: boolean): string {
  if (kind === "delete") return "tests.detail.deleteVersion.body";
  if (kind === "current") return "tests.detail.makeDefault.body";
  return draftAhead
    ? "tests.detail.restore.bodyReplace"
    : "tests.detail.restore.bodyCopy";
}

function actionError(
  error: Error | null,
  t: ReturnType<typeof useTranslation>["t"],
): string | null {
  if (error === null) return null;
  return error instanceof ApiError ? error.message : t("common.actionFailed");
}

function VersionDialog({
  action,
  draftAhead,
  pending,
  error,
  onCancel,
  onConfirm,
}: Readonly<{
  action: { kind: VersionAction; version: number } | null;
  draftAhead: boolean;
  pending: boolean;
  error: Error | null;
  onCancel: () => void;
  onConfirm: (input: { kind: VersionAction; version: number }) => void;
}>) {
  const { t } = useTranslation();
  const kind = action?.kind ?? "draft";
  const n = action?.version ?? 0;
  return (
    <ConfirmDialog
      open={action !== null}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
      title={t(DIALOG[kind].title, { n })}
      description={t(dialogBody(kind, draftAhead), { n })}
      confirmLabel={t(DIALOG[kind].confirm)}
      destructive={DIALOG[kind].destructive}
      pending={pending}
      error={actionError(error, t)}
      onConfirm={() => {
        if (action) onConfirm(action);
      }}
    />
  );
}
