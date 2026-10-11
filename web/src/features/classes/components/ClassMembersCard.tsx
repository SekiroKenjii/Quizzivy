import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { ChartColumn, Ellipsis, UserMinus } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { BulkActions } from "@/components/shared/BulkActions";
import { BulkSelectAll, BulkSelectRow } from "@/components/shared/BulkSelection";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput } from "@/components/shared/SearchInput";
import { Pager } from "@/components/shared/data/Pager";
import { useCan } from "@/features/auth/permissions";
import {
  fetchMembers,
  removeMember,
  type Class,
  type ClassMember,
} from "@/features/classes/api";
import { saveError } from "@/features/classes/classFields";
import { invalidateClassMembership } from "@/features/classes/invalidate";
import { scorePercent } from "@/features/students/api";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { dayMonth, formatDate, getDisplayTimeZone } from "@/lib/i18n/datetime";
import { formatInTimeZone } from "date-fns-tz";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

const PAGE_SIZES = [20, 50] as const;
const LOW_AVERAGE = 60;

const ROW =
  "grid items-center gap-3 px-4.5 grid-cols-[16px_minmax(0,2fr)_80px_32px] @[520px]/members:grid-cols-[16px_minmax(180px,2fr)_80px_70px_32px] @[620px]/members:grid-cols-[16px_minmax(180px,2fr)_120px_80px_70px_32px] @[720px]/members:grid-cols-[16px_minmax(180px,2fr)_120px_80px_80px_70px_32px]";

type Selectable = { id: string; name: string };

function menuTrigger(userId: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-member-menu="${userId}"]`);
}

/**
 * ClassMembersCard is the class's members, as the deck draws them: "Members
 * · {n}" with a search, then one row per member with a checkbox, the name
 * and email, how they joined (from 620px of the card), when (from 720px),
 * what they handed in and their average (from 520px, in danger ink under
 * 60%), and a menu with View results and Remove from class. Selecting rows
 * offers Remove selected. Removing anyone needs `canWrite`; without it there
 * are no checkboxes and the menu only views results. The footer says that
 * removing a student keeps their work.
 */
export function ClassMembersCard({
  klass,
  canWrite,
}: Readonly<{ klass: Class; canWrite: boolean }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const queryClient = useQueryClient();
  const canSeeResults = useCan("people.students.read");
  const bulk = useBulkSelection<Selectable>();
  const { params, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const search = useDebounced(query, 300).trim();
  const [size] = usePageSize(PAGE_SIZES);
  const [page] = usePage(search);
  const members = useQuery({
    queryKey: ["admin-class-members", klass.id, { q: search, page, size }],
    queryFn: ({ signal }) =>
      fetchMembers(
        klass.id,
        { limit: size, page, ...(search === "" ? {} : { q: search }) },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const returnTo = useRef<HTMLElement | null>(null);
  const [removing, setRemoving] = useState<ClassMember | null>(null);
  const remove = useMutation({
    mutationFn: (member: ClassMember) => removeMember(klass.id, member.userId),
    onSuccess: async (_, member) => {
      setRemoving(null);
      toast(t("classDetail.removedNamed", { name: member.fullName }));
      await invalidateClassMembership(queryClient, klass.id);
    },
  });

  const items = members.data?.items ?? [];
  const selectable = items.map((m) => ({ id: m.userId, name: m.fullName }));
  const total = members.data?.total ?? klass.studentCount;

  return (
    <section
      aria-labelledby="members-heading"
      className="bg-card shadow-card @container/members overflow-hidden rounded-xl border"
    >
      <div className="flex flex-wrap items-center justify-between gap-3 px-4.5 pt-4 pb-3">
        <h2
          id="members-heading"
          className="text-[0.9375rem] font-semibold tracking-[-0.01em] whitespace-nowrap"
        >
          {t("classDetail.membersHeading", { count: klass.studentCount })}
        </h2>
        <SearchInput
          className="[&_input]:bg-bg [&_input]:border-border max-w-60 min-w-0 flex-[0_1_240px] [&_input]:h-8.5 [&_input]:pl-8.25 [&_input]:text-[0.8125rem] [&_svg]:top-[9.5px] [&_svg]:size-3.75"
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("classDetail.searchMembers")}
        />
      </div>

      {canWrite ? (
        <BulkActions
          className="mx-3 mb-2.5"
          selected={[...bulk.selected.values()]}
          name={(item) => item.name}
          actions={[
            {
              label: t("classDetail.removeSelected"),
              description: t("classDetail.removeSelectedBody"),
              icon: UserMinus,
              run: (item) => removeMember(klass.id, item.id),
            },
          ]}
          onRemoved={bulk.remove}
          onClear={bulk.clear}
          onSettled={() => invalidateClassMembership(queryClient, klass.id)}
        />
      ) : null}

      <div role="table" aria-labelledby="members-heading">
        <div role="rowgroup">
          <div
            role="row"
            className={cn(
              ROW,
              "bg-muted text-muted-fg h-10 text-[0.78125rem] font-medium",
            )}
          >
            <span role="columnheader" className="flex">
              {canWrite ? <BulkSelectAll items={selectable} selection={bulk} /> : null}
            </span>
            <span role="columnheader">{t("classDetail.name")}</span>
            <span role="columnheader" className="hidden @[620px]/members:block">
              {t("classDetail.joinedVia")}
            </span>
            <span role="columnheader" className="hidden @[720px]/members:block">
              {t("classDetail.joinedAt")}
            </span>
            <span role="columnheader" className="text-right">
              {t("students.submitted")}
            </span>
            <span
              role="columnheader"
              className="hidden text-right @[520px]/members:block"
            >
              {t("students.average")}
            </span>
            <span role="columnheader">
              <span className="sr-only">{t("classDetail.actions")}</span>
            </span>
          </div>
        </div>
        <div role="rowgroup">
          {members.isPending ? <MembersSkeleton /> : null}
          {items.map((member) => (
            <MemberRow
              key={member.userId}
              member={member}
              locale={locale}
              selection={
                canWrite ? (
                  <BulkSelectRow
                    item={{ id: member.userId, name: member.fullName }}
                    name={member.fullName}
                    selection={bulk}
                  />
                ) : null
              }
              canWrite={canWrite}
              canSeeResults={canSeeResults}
              onRemove={() => {
                returnTo.current = menuTrigger(member.userId);
                remove.reset();
                setRemoving(member);
              }}
            />
          ))}
        </div>
      </div>

      {members.isError ? (
        <div className="border-t px-4.5 py-4">
          <LoadError error={members.error} onRetry={() => void members.refetch()}>
            {t("classDetail.membersFailed")}
          </LoadError>
        </div>
      ) : null}
      {members.data !== undefined && items.length === 0 ? (
        <p className="text-muted-fg border-t px-4.5 py-8 text-center text-sm">
          {search === ""
            ? t("classDetail.noMembers")
            : t("classDetail.noMemberMatches", { query: search })}
        </p>
      ) : null}
      {members.data !== undefined && total > PAGE_SIZES[0] ? (
        <Pager
          page={members.data.page}
          pageSize={members.data.pageSize}
          total={members.data.total}
          noun={(count) => t("classDetail.memberNoun", { count })}
          sizes={PAGE_SIZES}
        />
      ) : null}
      <p className="text-muted-fg border-t px-4.5 py-2.25 text-xs">
        {t("classDetail.removeKeepsWork")}
      </p>

      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => {
          if (open) return;
          setRemoving(null);
        }}
        title={t("classDetail.removeOneTitle", {
          name: removing?.fullName ?? "",
          className: klass.name,
        })}
        description={t("classDetail.removeConfirmBody")}
        confirmLabel={t("classDetail.removeConfirm")}
        destructive
        pending={remove.isPending}
        error={
          remove.isError ? saveError(remove.error, t("classDetail.removeFailed")) : null
        }
        returnFocus={returnTo}
        onConfirm={() => {
          if (removing !== null && !remove.isPending) remove.mutate(removing);
        }}
      />
    </section>
  );
}

function MemberRow({
  member,
  locale,
  selection,
  canWrite,
  canSeeResults,
  onRemove,
}: Readonly<{
  member: ClassMember;
  locale: ReturnType<typeof useLocale>;
  selection: React.ReactNode;
  canWrite: boolean;
  canSeeResults: boolean;
  onRemove: () => void;
}>) {
  const { t } = useTranslation();
  const percent = scorePercent(member.stats);
  return (
    <div
      role="row"
      className={cn(ROW, "hover:bg-muted min-h-14 border-t py-2 text-[0.84375rem]")}
    >
      <span role="cell" className="flex">
        {selection}
      </span>
      <span role="cell" className="flex min-w-0 items-center gap-2.5">
        <Avatar name={member.fullName} size="30" />
        <span className="min-w-0">
          <span className="block truncate font-medium">{member.fullName}</span>
          <span className="text-muted-fg block truncate text-xs">{member.email}</span>
        </span>
      </span>
      <span role="cell" className="hidden @[620px]/members:block">
        {member.joinedVia === "admin" ? (
          <span className="bg-primary text-primary-fg inline-flex h-5.5 items-center rounded-md px-2 text-xs font-medium whitespace-nowrap">
            {t("classDetail.viaAdmin")}
          </span>
        ) : (
          <span className="bg-muted inline-flex h-5.5 items-center rounded-md px-2 font-mono text-xs font-medium whitespace-nowrap">
            {t("classDetail.viaCode", { hint: member.joinCodeHint ?? "" })}
          </span>
        )}
      </span>
      <span
        role="cell"
        className="text-muted-fg hidden text-[0.8125rem] whitespace-nowrap @[720px]/members:block"
      >
        {joinedOn(member.joinedAt, locale)}
      </span>
      <span role="cell" className="text-right tabular-nums">
        {member.stats.submittedCount}
      </span>
      <span
        role="cell"
        className={cn(
          "hidden text-right tabular-nums @[520px]/members:block",
          percent === null && "text-muted-fg",
          percent !== null && percent < LOW_AVERAGE && "text-danger-ink",
        )}
      >
        {percent === null ? "—" : t("students.percent", { value: percent })}
      </span>
      <span role="cell" className="flex justify-end">
        {canWrite || canSeeResults ? (
          <MemberMenu
            member={member}
            canWrite={canWrite}
            canSeeResults={canSeeResults}
            onRemove={onRemove}
          />
        ) : null}
      </span>
    </div>
  );
}

function MemberMenu({
  member,
  canWrite,
  canSeeResults,
  onRemove,
}: Readonly<{
  member: ClassMember;
  canWrite: boolean;
  canSeeResults: boolean;
  onRemove: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          data-member-menu={member.userId}
          aria-label={t("classDetail.memberActions", { name: member.fullName })}
          className="hover:bg-hover data-[state=open]:bg-hover grid size-7.5 cursor-pointer place-items-center rounded-[0.4375rem]"
        >
          <Ellipsis aria-hidden="true" className="size-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52 data-[scale=deck]:w-55">
        {canSeeResults ? (
          <DropdownMenuItem asChild>
            <Link to={`/teacher/students?studentId=${member.userId}`}>
              <ChartColumn aria-hidden="true" />
              {t("classDetail.viewResults")}
            </Link>
          </DropdownMenuItem>
        ) : null}
        {canWrite ? (
          <>
            {canSeeResults ? <DropdownMenuSeparator /> : null}
            <DropdownMenuItem variant="destructive" onSelect={onRemove}>
              <UserMinus aria-hidden="true" />
              {t("classDetail.remove")}
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function joinedOn(at: string, locale: ReturnType<typeof useLocale>): string {
  const zone = getDisplayTimeZone();
  const year = (moment: string | Date) => formatInTimeZone(moment, zone, "yyyy");
  return year(at) === year(new Date()) ? dayMonth(at, locale) : formatDate(at, locale);
}

function MembersSkeleton() {
  const { t } = useTranslation();
  return (
    <div role="status" aria-label={t("common.loading")} className="flex flex-col">
      {[0, 1, 2].map((index) => (
        <div key={index} className="flex items-center gap-3 border-t px-4.5 py-3">
          <Skeleton className="size-7.5 rounded-full" />
          <Skeleton className="h-3.5 w-1/3" />
        </div>
      ))}
    </div>
  );
}
