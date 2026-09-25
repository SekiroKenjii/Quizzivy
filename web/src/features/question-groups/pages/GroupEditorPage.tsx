import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useBlocker, useBeforeUnload, useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { PageHeader } from "@/components/shared/PageHeader";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { useAuthStore } from "@/stores/auth";
import { ApiError } from "@/lib/api/errors";
import { createGroup, getGroup, updateGroup, type StoredGroup } from "../api";
import { groupIssue } from "../model";
import { independentBundle } from "../recovery";
import { GroupRecoveryGate, type RecoveryState } from "../components/GroupRecoveryGate";
import { useGroupEditor } from "../useGroupEditor";
import { GroupPreviewDialog } from "../components/GroupPreviewDialog";
import { GroupComposer } from "../components/GroupComposer";

export default function GroupEditorPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const owner = useAuthStore((state) => state.user?.id ?? "");
  const query = useQuery({
    queryKey: ["admin-group", id],
    queryFn: ({ signal }) => getGroup(id, signal),
  });
  if (query.isPending) return <ListSkeleton rows={8} />;
  if (query.isError)
    return (
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("groups.loadFailed")}
      </LoadError>
    );
  return (
    <GroupRecoveryGate key={`${owner}:${id}`} owner={owner} stored={query.data}>
      {(recovery) => <Editor stored={query.data} recovery={recovery} />}
    </GroupRecoveryGate>
  );
}

function Editor({
  stored,
  recovery,
}: Readonly<{ stored: StoredGroup; recovery: RecoveryState }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const client = useQueryClient();
  const [assets, setAssets] = useState(stored.assets);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState(false);
  const [copying, setCopying] = useState(false);
  const leaving = useRef(false);
  const [leaveBusy, setLeaveBusy] = useState(false);
  const id = stored.bundle.group.id;
  const editor = useGroupEditor({
    stored,
    recovered: recovery.recovery,
    scope: recovery.scope,
    ...(recovery.draft ? { createdAt: recovery.draft.createdAt } : {}),
    save: async (bundle, revision, testUpdatedAt) => {
      const saved = await updateGroup(id, {
        bundle,
        expectedRevision: revision,
        ...(testUpdatedAt ? { expectedTestUpdatedAt: testUpdatedAt } : {}),
      });
      setAssets((previous) => [
        ...new Map(
          [...previous, ...saved.assets].map((asset) => [asset.id, asset]),
        ).values(),
      ]);
      client.setQueryData(["admin-group", id], saved);
      void client.invalidateQueries({ queryKey: ["admin-groups"] });
      return saved;
    },
  });
  const blocker = useBlocker(
    ({ nextLocation }) =>
      editor.dirty && !leaving.current && nextLocation.pathname !== "/login",
  );
  useBeforeUnload((event) => {
    if (editor.dirty) event.preventDefault();
  });
  const save = async () => {
    setError(null);
    try {
      await editor.saveNow();
      return true;
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("groups.saveFailed"));
      return false;
    }
  };
  async function saveCopy() {
    const issue = groupIssue(editor.bundle);
    if (issue) {
      setError(t(issue));
      return;
    }
    setCopying(true);
    setError(null);
    try {
      const saved = await createGroup({ bundle: independentBundle(editor.bundle) });
      await editor.discardLocal();
      client.setQueryData(["admin-group", saved.bundle.group.id], saved);
      void client.invalidateQueries({ queryKey: ["admin-groups"] });
      leaving.current = true;
      void navigate(`/admin/question-bank/groups/${saved.bundle.group.id}`);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("groups.saveFailed"));
    } finally {
      setCopying(false);
    }
  }
  const refresh = () => {
    void getGroup(id)
      .then((fresh) => setAssets((previous) => mergeAssets(previous, fresh.assets)))
      .catch(() => setError(t("groups.loadFailed")));
  };

  return (
    <div className="flex min-w-0 flex-col gap-5">
      <PageHeader
        title={editor.bundle.group.title || t("groups.newGroup")}
        backTo="/admin/question-bank/groups"
        actions={
          <>
            <Button variant="outline" size="sm" onClick={() => setPreview(true)}>
              {t("builder.previewAsStudent")}
            </Button>
            <Button
              size="sm"
              disabled={
                copying || editor.status.kind === "saving" || stored.archivedAt !== null
              }
              onClick={() => {
                if (editor.copyRequired) void saveCopy();
                else void save();
              }}
            >
              {t(editor.copyRequired ? "groups.saveCopy" : "common.save")}
            </Button>
          </>
        }
      />
      <p role="status" className="text-muted-foreground text-sm">
        {t(saveLabel(editor))}
      </p>
      {editor.copyRequired ? (
        <Alert>
          <AlertTitle>{t("groups.conflictTitle")}</AlertTitle>
          <AlertDescription>{t("groups.conflictBody")}</AlertDescription>
        </Alert>
      ) : null}
      {stored.archivedAt ? (
        <Alert>
          <AlertDescription>{t("groups.archivedHint")}</AlertDescription>
        </Alert>
      ) : null}
      {error || editor.status.kind === "failed" ? (
        <Alert>
          <AlertDescription role="alert">
            {error ??
              (editor.status.kind === "failed"
                ? editor.status.message || t("groups.saveFailed")
                : "")}
          </AlertDescription>
        </Alert>
      ) : null}
      {preview ? (
        <GroupPreviewDialog
          bundle={editor.bundle}
          assets={assets}
          onClose={() => setPreview(false)}
          onRefresh={refresh}
        />
      ) : null}
      <fieldset disabled={stored.archivedAt !== null || copying} className="min-w-0">
        {stored.archivedAt === null ? (
          <GroupComposer
            bundle={editor.bundle}
            assets={assets}
            onChange={editor.change}
            onAsset={(asset) =>
              setAssets((previous) => [
                ...previous.filter((item) => item.id !== asset.id),
                asset,
              ])
            }
            onRefresh={refresh}
          />
        ) : null}
      </fieldset>
      <ConfirmDialog
        open={blocker.state === "blocked"}
        onOpenChange={(open) => {
          if (!open && blocker.state === "blocked") blocker.reset();
        }}
        title={t("groups.leaveTitle")}
        description={t("groups.leaveBody")}
        confirmLabel={t("groups.saveAndLeave")}
        disabled={editor.copyRequired || leaveBusy}
        pending={leaveBusy}
        error={error}
        onConfirm={() => {
          setLeaveBusy(true);
          void save()
            .then((ok) => {
              if (ok && blocker.state === "blocked") blocker.proceed();
            })
            .finally(() => setLeaveBusy(false));
        }}
      >
        <Button
          variant="outline"
          disabled={leaveBusy}
          onClick={() => {
            setLeaveBusy(true);
            void editor
              .persist()
              .then((ok) => {
                if (ok && blocker.state === "blocked") blocker.proceed();
                else setError(t("groups.localUnavailable"));
              })
              .finally(() => setLeaveBusy(false));
          }}
        >
          {t("groups.keepLocalAndLeave")}
        </Button>
      </ConfirmDialog>
    </div>
  );
}

function saveLabel(editor: ReturnType<typeof useGroupEditor>): string {
  if (!editor.dirty) return "groups.serverSaved";
  if (editor.status.kind === "saving") return "common.saving";
  if (editor.local === "stored") return "groups.localSaved";
  if (editor.local === "unavailable") return "groups.localUnavailable";
  return "groups.unsaved";
}

function mergeAssets(
  previous: StoredGroup["assets"],
  incoming: StoredGroup["assets"],
): StoredGroup["assets"] {
  return [
    ...new Map([...previous, ...incoming].map((asset) => [asset.id, asset])).values(),
  ];
}
