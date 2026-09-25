import { useEffect, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { useAuthStore } from "@/stores/auth";
import { ApiError } from "@/lib/api/errors";
import {
  createGroup,
  getGroup,
  type GroupBundle,
  type StoredGroup,
} from "@/features/question-groups/api";
import { GroupComposer } from "@/features/question-groups/components/GroupComposer";
import {
  GroupRecoveryGate,
  type RecoveryState,
} from "@/features/question-groups/components/GroupRecoveryGate";
import { groupIssue } from "@/features/question-groups/model";
import { independentBundle } from "@/features/question-groups/recovery";
import { useGroupEditor } from "@/features/question-groups/useGroupEditor";
import type { AutosaveStatus } from "../useAutosave";

export interface GroupPaneBridge {
  id: string;
  advanceOwner: (moved: boolean, testUpdatedAt: string) => void;
  persist: () => Promise<boolean>;
}
interface PaneProps {
  id: string;
  selectedQuestionId: string | null;
  coordinate: (operation: () => Promise<void>) => Promise<void>;
  save: (bundle: GroupBundle, revision: number) => Promise<StoredGroup>;
  onChange: (bundle: GroupBundle) => void;
  onSelectedQuestionChange: (id: string | null) => void;
  flushRef: RefObject<(() => Promise<void>) | null>;
  retryRef: RefObject<(() => void) | null>;
  bridgeRef: RefObject<GroupPaneBridge | null>;
  onStatus: (status: AutosaveStatus) => void;
}

export function BuilderGroupPane(props: Readonly<PaneProps>) {
  const { t } = useTranslation();
  const owner = useAuthStore((state) => state.user?.id ?? "");
  const query = useQuery({
    queryKey: ["admin-group", props.id],
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    queryFn: ({ signal }) => getGroup(props.id, signal),
  });
  if (query.isPending) return <ListSkeleton rows={6} />;
  if (query.isError)
    return (
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("groups.loadFailed")}
      </LoadError>
    );
  return (
    <GroupRecoveryGate owner={owner} stored={query.data}>
      {(recovery) => <GroupForm {...props} stored={query.data} recovery={recovery} />}
    </GroupRecoveryGate>
  );
}

function GroupForm({
  stored,
  recovery,
  selectedQuestionId,
  onSelectedQuestionChange,
  coordinate,
  save,
  onChange,
  flushRef,
  retryRef,
  bridgeRef,
  onStatus,
}: Readonly<PaneProps & { stored: StoredGroup; recovery: RecoveryState }>) {
  const { t } = useTranslation();
  const [assets, setAssets] = useState(stored.assets);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [copying, setCopying] = useState(false);
  const editor = useGroupEditor({
    stored,
    recovered: recovery.recovery,
    scope: recovery.scope,
    ...(recovery.draft ? { createdAt: recovery.draft.createdAt } : {}),
    coordinate,
    save: async (bundle, revision) => {
      const saved = await save(bundle, revision);
      setAssets((current) => [
        ...new Map(
          [...current, ...saved.assets].map((asset) => [asset.id, asset]),
        ).values(),
      ]);
      return saved;
    },
  });
  const { saveNow, advanceOwner, persist, status, copyRequired } = editor;
  useEffect(() => onChange(editor.bundle), [onChange, editor.bundle]);
  useEffect(() => {
    flushRef.current = saveNow;
    retryRef.current = () => void saveNow().catch(() => undefined);
    bridgeRef.current = { id: stored.bundle.group.id, advanceOwner, persist };
    return () => {
      flushRef.current = null;
      retryRef.current = null;
      bridgeRef.current = null;
    };
  }, [
    saveNow,
    advanceOwner,
    persist,
    flushRef,
    retryRef,
    bridgeRef,
    stored.bundle.group.id,
  ]);
  useEffect(() => {
    if (copyRequired) onStatus({ kind: "stale" });
    else if (editor.dirty && (status.kind === "idle" || status.kind === "saved"))
      onStatus({ kind: "dirty" });
    else onStatus(status);
  }, [copyRequired, editor.dirty, onStatus, status]);

  async function saveBankCopy() {
    const issue = groupIssue(editor.bundle);
    if (issue) {
      setError(t(issue));
      return;
    }
    setCopying(true);
    setError(null);
    try {
      const result = await createGroup({ bundle: independentBundle(editor.bundle) });
      setCopied(result.bundle.group.id);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("groups.saveFailed"));
    } finally {
      setCopying(false);
    }
  }
  function refresh() {
    void getGroup(stored.bundle.group.id)
      .then((fresh) => setAssets((current) => mergeAssets(current, fresh.assets)))
      .catch(() => setError(t("groups.loadFailed")));
  }
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p role="status" className="text-muted-foreground text-sm">
          {t(localSaveLabel(editor))}
        </p>
        <Button
          variant="outline"
          size="sm"
          disabled={copying}
          onClick={() => void saveBankCopy()}
        >
          {t("builder.saveGroupToBank")}
        </Button>
      </div>
      {copied ? (
        <p role="status" className="text-sm">
          {t("builder.groupSavedToBank")}{" "}
          <Link className="underline" to={`/admin/question-bank/groups/${copied}`}>
            {t("common.view")}
          </Link>
        </p>
      ) : null}
      {copyRequired ? (
        <Alert>
          <AlertTitle>{t("groups.conflictTitle")}</AlertTitle>
          <AlertDescription>{t("groups.conflictBody")}</AlertDescription>
        </Alert>
      ) : null}
      {error || status.kind === "failed" ? (
        <Alert>
          <AlertDescription role="alert">
            {error ??
              (status.kind === "failed"
                ? status.message || t("groups.saveFailed")
                : "")}
          </AlertDescription>
        </Alert>
      ) : null}
      <GroupComposer
        embedded
        bundle={editor.bundle}
        assets={assets}
        selectedQuestionId={selectedQuestionId ?? undefined}
        onSelectedQuestionChange={onSelectedQuestionChange}
        onChange={editor.change}
        onAsset={(asset) =>
          setAssets((current) => [
            ...current.filter((item) => item.id !== asset.id),
            asset,
          ])
        }
        onRefresh={refresh}
      />
    </div>
  );
}

function localSaveLabel(editor: ReturnType<typeof useGroupEditor>): string {
  if (!editor.dirty) return "groups.serverSaved";
  if (editor.local === "stored") return "groups.localSaved";
  if (editor.local === "unavailable") return "groups.localUnavailable";
  return "groups.unsaved";
}
function mergeAssets(current: StoredGroup["assets"], incoming: StoredGroup["assets"]) {
  return [
    ...new Map([...current, ...incoming].map((asset) => [asset.id, asset])).values(),
  ];
}
