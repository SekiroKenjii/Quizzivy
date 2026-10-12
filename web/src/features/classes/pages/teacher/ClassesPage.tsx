import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DeckScale } from "@/components/ui/deck-scale";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { Callout } from "@/components/shared/Callout";
import { CardGrid } from "@/components/shared/CardGrid";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { DeleteItemDialog } from "@/components/shared/DeleteItemButton";
import { EmptyState, LoadError, QueryStates } from "@/components/shared/ListState";
import { Pager } from "@/components/shared/data/Pager";
import { useCan } from "@/features/auth/permissions";
import {
  deleteClass,
  fetchClasses,
  rotateJoinCode,
  updateClass,
  type ClassListItem,
} from "@/features/classes/api";
import { ArchiveClassDialog } from "@/features/classes/components/ArchiveClassDialog";
import { ClassCard } from "@/features/classes/components/ClassCard";
import { EditClassDialog } from "@/features/classes/components/EditClassDialog";
import { NewClassDialog } from "@/features/classes/components/NewClassDialog";
import { ShowQrDialog } from "@/features/classes/components/ShowQrDialog";
import { invalidateClass } from "@/features/classes/invalidate";
import { useCopyJoinLink } from "@/features/classes/joinLink";
import { useMarkNotificationsRead } from "@/features/notifications/useMarkNotificationsRead";
import { useNotificationList } from "@/features/notifications/useNotificationList";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { ApiError } from "@/lib/api/errors";
import { formatDate, useDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

const PAGE_SIZES = [24, 48] as const;
const NEW_CODE = { expiresInDays: 30, maxUses: 40 } as const;
type Tab = "active" | "archived";

function menuTrigger(id: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-class-menu="${id}"]`);
}

function survivor(id: string): HTMLElement | null {
  return menuTrigger(id) ?? document.querySelector<HTMLElement>("main");
}

function message(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}

/**
 * ClassesPage is the teacher's classes, as the deck's Classes screen draws
 * them: "Students join with a code or a link.", New class, and one card per
 * class with its join code in full. Active and Archived split the list as
 * Admin › Classes draws it, 24 or 48 cards a page. The codes come from the
 * list itself (`withCodes`), asked for only by a reader who may change
 * classes and dropped from memory when the page goes. While the notice that
 * the join codes were replaced is unread, a callout says so; dismissing it
 * marks it read. New code and Create code give focus back to the card's
 * menu, which outlives the footer button that may have opened them; once a
 * restored or deleted card has left the list, focus goes to the page.
 */
export default function ClassesPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const canWrite = useCan("teaching.classes.write");
  const { params, setFilter } = useListFilters();
  const tab: Tab = params.get("status") === "archived" ? "archived" : "active";
  const [size] = usePageSize(PAGE_SIZES);
  const [page] = usePage(tab);
  const classes = useQuery({
    queryKey: ["teacher-classes", { status: tab, page, size, withCodes: canWrite }],
    queryFn: ({ signal }) =>
      fetchClasses({ status: tab, page, limit: size, withCodes: canWrite }, signal),
    placeholderData: keepPreviousData,
    gcTime: 0,
  });
  const [now] = useState(() => Date.now());
  const newButton = useRef<HTMLButtonElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<ClassListItem | null>(null);
  const [showing, setShowing] = useState<{ name: string; code: string } | null>(null);
  const [recoding, setRecoding] = useState<ClassListItem | null>(null);
  const [archiving, setArchiving] = useState<ClassListItem | null>(null);
  const [deleting, setDeleting] = useState<ClassListItem | null>(null);
  const copyLink = useCopyJoinLink();
  const settling = useRef<string | null>(null);
  const refreshedAt = classes.dataUpdatedAt;

  useEffect(() => {
    const id = settling.current;
    if (id === null) return;
    settling.current = null;
    const active = document.activeElement;
    if (active === null || active === document.body) survivor(id)?.focus();
  }, [refreshedAt]);

  const fromMenu =
    (open: (klass: ClassListItem) => void) =>
    (klass: ClassListItem, from?: HTMLElement) => {
      returnTo.current = from ?? menuTrigger(klass.id);
      open(klass);
    };
  const showQr = (klass: ClassListItem, code: string, from?: HTMLElement) => {
    returnTo.current = from ?? menuTrigger(klass.id);
    setShowing({ name: klass.name, code });
  };

  const recode = useMutation({
    mutationFn: (klass: ClassListItem) => rotateJoinCode(klass.id, NEW_CODE),
    onSuccess: async (issued, klass) => {
      setRecoding(null);
      toast(t("classes.codeRenewedWith", { code: issued.code }));
      await invalidateClass(queryClient, klass.id);
    },
  });
  const restore = useMutation({
    mutationFn: (klass: ClassListItem) => updateClass(klass.id, { archived: false }),
    onSuccess: async (_, klass) => {
      toast(t("classes.restored"));
      settling.current = klass.id;
      await invalidateClass(queryClient, klass.id);
    },
    onError: (cause) => toast.error(message(cause, t("classes.restoreFailed"))),
  });

  const facets = classes.data?.facets;
  const counts = {
    active: facets === undefined ? undefined : facets.all - facets.archived,
    archived: facets?.archived,
  };
  const nothingYet = facets !== undefined && facets.all === 0;

  const results = (data: NonNullable<typeof classes.data>) => {
    if (data.total > 0)
      return (
        <>
          <CardGrid
            label={t("teacherShell.nav.classes")}
            items={data.items}
            itemKey={(klass) => klass.id}
            min={300}
          >
            {(klass) => (
              <ClassCard
                klass={klass}
                canWrite={canWrite}
                now={now}
                onCopyLink={copyLink}
                onShowQr={showQr}
                onEdit={fromMenu(setEditing)}
                onNewCode={(klass) => {
                  returnTo.current = menuTrigger(klass.id);
                  setRecoding(klass);
                }}
                onArchive={fromMenu(setArchiving)}
                onRestore={(klass) => {
                  if (!restore.isPending) restore.mutate(klass);
                }}
                onDelete={fromMenu(setDeleting)}
              />
            )}
          </CardGrid>
          <Pager
            page={data.page}
            pageSize={data.pageSize}
            total={data.total}
            noun={(count) => t("classes.noun", { count })}
            sizes={PAGE_SIZES}
            frame="plain"
          />
        </>
      );
    if (nothingYet && canWrite)
      return (
        <EmptyState
          hint={t("classes.emptyHint")}
          action={
            <Button size="sm" onClick={() => setCreating(true)}>
              <Plus aria-hidden="true" />
              {t("classes.new")}
            </Button>
          }
        >
          {t("classes.empty")}
        </EmptyState>
      );
    if (nothingYet) return <EmptyState>{t("classes.empty")}</EmptyState>;
    return (
      <EmptyState>
        {t(tab === "archived" ? "classes.noArchived" : "classes.noActive")}
      </EmptyState>
    );
  };

  return (
    <DeckScale className="mx-auto flex w-full max-w-[1320px] min-w-0 flex-col gap-4">
      <PageHead
        title={t("teacherShell.nav.classes")}
        description={t("classes.pageLead")}
        actions={
          canWrite ? (
            <Button ref={newButton} onClick={() => setCreating(true)}>
              <Plus aria-hidden="true" />
              {t("classes.new")}
            </Button>
          ) : undefined
        }
      />

      <ReplacedCodesCallout />

      {nothingYet ? null : (
        <Segmented
          className="max-w-full self-start"
          scroll
          label={t("classes.statusFilter")}
          value={tab}
          options={[
            { value: "active", label: t("classes.tabs.active"), count: counts.active },
            {
              value: "archived",
              label: t("classes.tabs.archived"),
              count: counts.archived,
            },
          ]}
          onChange={(value) =>
            setFilter("status", value === "archived" ? "archived" : null)
          }
        />
      )}

      {classes.data === undefined ? (
        <QueryStates
          query={classes}
          skeleton={<ClassesSkeleton />}
          failed={t("classes.loadFailed")}
        >
          {results}
        </QueryStates>
      ) : (
        <>
          {classes.isError ? (
            <LoadError error={classes.error} onRetry={() => void classes.refetch()}>
              {t("classes.loadFailed")}
            </LoadError>
          ) : null}
          {results(classes.data)}
        </>
      )}

      <NewClassDialog
        open={creating}
        onOpenChange={setCreating}
        returnFocus={newButton}
      />
      <EditClassDialog
        klass={editing}
        onClose={() => setEditing(null)}
        returnFocus={returnTo}
      />
      <ShowQrDialog
        target={showing}
        onClose={() => setShowing(null)}
        returnFocus={returnTo}
      />
      <ConfirmDialog
        open={recoding !== null}
        onOpenChange={(open) => {
          if (open) return;
          recode.reset();
          setRecoding(null);
        }}
        title={t(
          recoding?.joinCode ? "classes.newCodeTitle" : "classes.createCodeTitle",
        )}
        description={t(
          recoding?.joinCode ? "classes.newCodeBody" : "classes.createCodeBody",
        )}
        confirmLabel={t(
          recoding?.joinCode ? "classes.newCodeShort" : "classes.createCode",
        )}
        pending={recode.isPending}
        error={
          recode.isError ? message(recode.error, t("classes.newCodeFailed")) : null
        }
        returnFocus={returnTo}
        onConfirm={() => {
          if (recoding !== null && !recode.isPending) recode.mutate(recoding);
        }}
      />
      <ArchiveClassDialog
        klass={archiving}
        onOpenChange={(open) => {
          if (!open) setArchiving(null);
        }}
        returnFocus={returnTo}
      />
      <DeleteItemDialog
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
        name={deleting?.name ?? ""}
        returnFocus={returnTo}
        onDelete={async () => {
          if (deleting === null) return;
          await deleteClass(deleting.id);
          settling.current = deleting.id;
        }}
        onDeleted={() =>
          queryClient.invalidateQueries({ queryKey: ["teacher-classes"] })
        }
      />
    </DeckScale>
  );
}

function ReplacedCodesCallout() {
  const { t } = useTranslation();
  const locale = useLocale();
  const list = useNotificationList("teacher");
  const mark = useMarkNotificationsRead("teacher");
  const [dismissed, setDismissed] = useState<string | null>(null);
  const notice = list.items.find(
    (item) =>
      item.kind === "join_codes.rotated" &&
      item.readAt === null &&
      item.id !== dismissed,
  );
  if (notice === undefined) return null;
  return (
    <Callout
      tone="warning"
      onDismiss={() => {
        setDismissed(notice.id);
        mark.mutate([notice.id]);
      }}
    >
      {t("classes.codesReplaced", { date: formatDate(notice.createdAt, locale) })}
    </Callout>
  );
}

function ClassesSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="grid grid-cols-[repeat(auto-fill,minmax(min(300px,100%),1fr))] gap-3"
    >
      {Array.from({ length: 4 }, (_, index) => (
        <Skeleton key={index} className="h-48 rounded-xl" />
      ))}
    </div>
  );
}
