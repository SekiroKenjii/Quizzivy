import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Badge } from "@/components/ui/badge";
import { toast } from "@/components/ui/sonner";
import { useState } from "react";
import { History } from "lucide-react";
import { PageAside } from "@/components/shared/PageAside";
import { useTranslation } from "react-i18next";
import {
  Link,
  useLocation,
  useParams,
  useSearchParams,
  useNavigate,
} from "react-router";
import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import {
  getTest,
  listVersions,
  previewTest,
  createDraftFromTestVersion,
  deleteTestVersion,
  setCurrentTestVersion,
} from "@/features/tests/api";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatDateTime } from "@/lib/i18n/datetime";
import { ApiError } from "@/lib/api/errors";
import { ListSkeleton, LoadError, QueryStates } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { StatusBadge } from "@/components/shared/StatusBadge";

/**
 * §8's test detail: what a student would receive, and the history of what they
 * have received before.
 */
export default function TestDetailPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const locale = useLocale();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [action, setAction] = useState<{
    kind: "draft" | "delete" | "current";
    version: number;
  } | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const { hash } = useLocation();
  const [historyOpen, setHistoryOpen] = useState(hash === "#versions");
  const [params, setParams] = useSearchParams();
  const requestedVersion = Number(params.get("version"));
  const version =
    Number.isSafeInteger(requestedVersion) && requestedVersion > 0
      ? requestedVersion
      : undefined;

  const test = useQuery({
    queryKey: ["admin-test", id],
    queryFn: ({ signal }) => getTest(id, signal),
  });
  const versions = useQuery({
    queryKey: ["admin-test-versions", id],
    queryFn: ({ signal }) => listVersions(id, signal),
  });
  const preview = useQuery({
    queryKey: ["admin-test-preview", id, version],
    queryFn: ({ signal }) => previewTest(id, version, signal),
    retry: false,
  });

  const change = useMutation({
    mutationFn: async (input: NonNullable<typeof action>) => {
      if (!test.data) throw new Error("Test unavailable");
      if (input.kind === "delete") return deleteTestVersion(id, input.version);
      if (input.kind === "draft")
        return createDraftFromTestVersion(test.data, input.version);
      return setCurrentTestVersion(test.data, input.version);
    },
    onSuccess: async (_, input) => {
      setAction(null);
      if (input.kind === "delete" && version === input.version) setParams({});
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["admin-test", id] }),
        queryClient.invalidateQueries({ queryKey: ["admin-test-versions", id] }),
        queryClient.invalidateQueries({ queryKey: ["admin-test-preview", id] }),
        queryClient.invalidateQueries({ queryKey: ["admin-tests"] }),
        queryClient.invalidateQueries({ queryKey: ["admin-questions"] }),
      ]);
      toast(t("tests.versionActionSaved"));
      if (input.kind === "draft") void navigate(`/admin/tests/${id}/edit`);
    },
    onError: (cause) =>
      setActionError(
        cause instanceof ApiError ? cause.message : t("common.actionFailed"),
      ),
  });

  if (test.isPending) {
    return <ListSkeleton rows={6} />;
  }
  if (test.isError) {
    return (
      <LoadError error={test.error} onRetry={() => void test.refetch()}>
        {t("tests.detailFailed")}
      </LoadError>
    );
  }

  const notPublished =
    preview.error instanceof ApiError && preview.error.code === "TEST_NOT_PUBLISHED";

  return (
    <>
      <PageHeader
        title={test.data.title}
        backTo="/admin/tests"
        meta={<StatusBadge kind="test" status={test.data.status} />}
        actions={
          <>
            <Button
              size="sm"
              variant="outline"
              className="lg:hidden"
              onClick={() => setHistoryOpen(true)}
            >
              <History aria-hidden="true" />
              {t("tests.versionHistory")}
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to={`/admin/tests/${id}/edit`}>{t("tests.openBuilder")}</Link>
            </Button>
          </>
        }
      />

      <div className="space-y-4">
        <div>
          <h2 className="text-[0.9375rem] font-semibold tracking-[-0.01em]">
            {t("tests.preview")}
          </h2>
          <p className="text-muted-foreground mt-0.5 text-sm">
            {preview.data
              ? t("tests.previewOf", { n: preview.data.version })
              : t("tests.previewNote")}
          </p>
        </div>

        {notPublished && !preview.isPending ? (
          <div className="space-y-3">
            <p className="text-muted-foreground text-sm">{t("tests.notPublished")}</p>
            <Button asChild size="sm">
              <Link to={`/admin/tests/${id}/edit`}>{t("tests.openBuilder")}</Link>
            </Button>
          </div>
        ) : (
          <QueryStates
            query={preview}
            skeleton={<ListSkeleton rows={4} />}
            failed={t("tests.previewFailed")}
          >
            {(data) => (
              <StudentPreviewPane
                questions={data.questions}
                sections={data.sections ?? []}
                groups={data.groups ?? []}
                onRetryMedia={() => void preview.refetch()}
              />
            )}
          </QueryStates>
        )}
      </div>
      <PageAside
        label={t("tests.versionHistory")}
        hideBelow="lg"
        sheet={{ open: historyOpen, onOpenChange: setHistoryOpen }}
      >
        <h2 className="text-sm font-semibold">{t("tests.versionHistory")}</h2>
        <VersionHistory
          query={versions}
          locale={locale}
          selected={version ?? test.data.currentVersion}
          current={test.data.currentVersion}
          pending={change.isPending}
          onAction={(kind, version) => {
            setActionError(null);
            setAction({ kind, version });
          }}
          onSelect={(value) => {
            setParams((current) => {
              const next = new URLSearchParams(current);
              next.set("version", String(value));
              return next;
            });
            setHistoryOpen(false);
          }}
        />
      </PageAside>
      <ConfirmDialog
        open={action !== null}
        onOpenChange={(open) => {
          if (!open && !change.isPending) setAction(null);
        }}
        title={t(`tests.versionActions.${action?.kind ?? "draft"}`)}
        description={t(`tests.versionActionBodies.${action?.kind ?? "draft"}`)}
        confirmLabel={t(`tests.versionActions.${action?.kind ?? "draft"}`)}
        destructive={action?.kind === "delete"}
        pending={change.isPending}
        onConfirm={() => action && change.mutate(action)}
      >
        <p className="text-sm">{t("tests.versionNumber", { n: action?.version })}</p>
        {actionError ? (
          <p role="alert" className="text-destructive text-sm">
            {actionError}
          </p>
        ) : null}
      </ConfirmDialog>
    </>
  );
}

function VersionHistory({
  current,
  pending,
  onAction,
  query,
  locale,
  selected,
  onSelect,
}: Readonly<{
  current: number;
  pending: boolean;
  onAction: (kind: "draft" | "delete" | "current", version: number) => void;
  query: UseQueryResult<Awaited<ReturnType<typeof listVersions>>>;
  locale: Locale;
  selected: number;
  onSelect: (version: number) => void;
}>) {
  const { t } = useTranslation();
  if (query.isPending) {
    return <ListSkeleton rows={2} />;
  }
  if (query.isError) {
    return (
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("tests.loadFailed")}
      </LoadError>
    );
  }
  if (query.data.items.length === 0) {
    return <p className="text-muted-foreground text-sm">{t("tests.noVersions")}</p>;
  }
  return (
    <ol className="space-y-3">
      {query.data.items.map((version) => (
        <li key={version.id} className="text-sm">
          <button
            type="button"
            aria-pressed={selected === version.version}
            onClick={() => onSelect(version.version)}
            className="hover:bg-accent aria-pressed:bg-accent flex w-full items-baseline justify-between gap-3 rounded-md p-2 text-left"
          >
            <span className="font-medium tabular-nums">
              <span>{t("tests.versionNumber", { n: version.version })}</span>
              {version.version === current ? (
                <Badge variant="outline" className="ml-2">
                  {t("tests.defaultVersion")}
                </Badge>
              ) : null}
            </span>
            <span className="text-muted-foreground text-xs tabular-nums">
              {version.questionCount} · {version.totalPoints}
            </span>
          </button>
          <p className="text-muted-foreground px-2 text-xs">
            {formatDateTime(version.publishedAt, locale)} · {version.publishedBy}
          </p>
          <div className="mt-2 flex flex-wrap gap-1 px-2">
            <Button
              size="xs"
              variant="outline"
              disabled={pending}
              onClick={() => onAction("draft", version.version)}
            >
              {t("tests.versionActions.draft")}
            </Button>
            {version.version !== current ? (
              <>
                <Button
                  size="xs"
                  variant="ghost"
                  disabled={pending}
                  onClick={() => onAction("current", version.version)}
                >
                  {t("tests.versionActions.current")}
                </Button>
                <Button
                  size="xs"
                  variant="ghost"
                  disabled={pending}
                  onClick={() => onAction("delete", version.version)}
                >
                  {t("tests.versionActions.delete")}
                </Button>
              </>
            ) : null}
          </div>
        </li>
      ))}
    </ol>
  );
}
