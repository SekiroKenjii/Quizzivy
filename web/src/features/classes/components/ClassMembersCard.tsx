import { useMemo, useRef, useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import { formatInTimeZone } from "date-fns-tz";
import { ChartColumn, UserMinus } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/sonner";
import { BulkActions } from "@/components/shared/BulkActions";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { LoadError } from "@/components/shared/ListState";
import { SearchInput } from "@/components/shared/SearchInput";
import {
  DataTable,
  type DataColumn,
  type DataTableMenuContext,
} from "@/components/shared/data/DataTable";
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
import { useJoinCodesOff } from "@/features/organization/api";
import { scorePercent } from "@/features/students/api";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import { useContentBand } from "@/layouts/shell/contentWidth";
import { dayMonth, dayMonthYear, getDisplayTimeZone } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

const PAGE_SIZES = [20, 50] as const;
const LOW_AVERAGE = 60;
const MEMBER_BANDS = [520, 620, 720, 1000, 1080] as const;
const MEMBER_SETS: readonly ReadonlySet<string>[] = [
  new Set(["name", "submitted"]),
  new Set(["name", "submitted", "average"]),
  new Set(["name", "via", "submitted", "average"]),
  new Set(["name", "via", "at", "submitted", "average"]),
  new Set(["name", "via", "submitted", "average"]),
  new Set(["name", "via", "at", "submitted", "average"]),
];
const ALL_COLUMNS: ReadonlySet<string> = new Set([
  "name",
  "via",
  "at",
  "submitted",
  "average",
]);

type Member = ClassMember & { id: string };
type MemberPage = Awaited<ReturnType<typeof fetchMembers>>;
type Locale = ReturnType<typeof useLocale>;
type Translate = ReturnType<typeof useTranslation>["t"];

/**
 * ClassMembersCard is the class's members, as the deck draws them: "Members
 * · {n}" with a search, then a DataTable with one row per member: a
 * checkbox, the name and email, how they joined, when, what they handed in
 * and their average (in danger ink under 60%), and a menu with View results
 * and Remove from class. The columns follow the deck's content widths: the
 * card is the whole width below 1000px and two thirds of it from there, so
 * Joined via shows from 620 and again from 1000, and Joined from 720 and
 * again from 1080. A selected row is tinted, and selecting offers Remove
 * selected. Removing anyone needs `canWrite`; without it there are no
 * checkboxes and the menu only views results. Once a member is removed from
 * their row's menu, focus goes to the search, since the row is gone. The
 * footer says that removing a student keeps their work.
 */
export function ClassMembersCard({
  klass,
  canWrite,
}: Readonly<{ klass: Class; canWrite: boolean }>) {
  const { t } = useTranslation();
  const joinOff = useJoinCodesOff();
  const locale = useLocale();
  const queryClient = useQueryClient();
  const canSeeResults = useCan("people.students.read");
  const bulk = useBulkSelection<Member>();
  const band = useContentBand(MEMBER_BANDS);
  const { params, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const search = useDebounced(query, 300).trim();
  const [size] = usePageSize(PAGE_SIZES);
  const [page] = usePage(search);
  const members = useQuery({
    queryKey: ["teacher-class-members", klass.id, { q: search, page, size }],
    queryFn: ({ signal }) =>
      fetchMembers(
        klass.id,
        { limit: size, page, ...(search === "" ? {} : { q: search }) },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const rows = useMemo(
    () =>
      (members.data?.items ?? []).map((member) => ({ ...member, id: member.userId })),
    [members.data],
  );
  const columns = useMemo(() => memberColumns(t, locale), [t, locale]);
  const card = useRef<HTMLElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);
  const [removing, setRemoving] = useState<Member | null>(null);
  const remove = useMutation({
    mutationFn: (member: Member) => removeMember(klass.id, member.userId),
    onSuccess: async (_, member) => {
      returnTo.current = card.current?.querySelector("input") ?? null;
      setRemoving(null);
      toast(t("classDetail.removedNamed", { name: member.fullName }));
      await invalidateClassMembership(queryClient, klass.id);
    },
  });

  const askToRemove = (
    member: Member,
    trigger: RefObject<HTMLButtonElement | null>,
  ) => {
    returnTo.current = trigger.current;
    remove.reset();
    setRemoving(member);
  };
  const menu =
    canWrite || canSeeResults
      ? (member: Member, { triggerRef }: DataTableMenuContext) => (
          <MemberMenuItems
            member={member}
            canWrite={canWrite}
            canSeeResults={canSeeResults}
            onRemove={() => askToRemove(member, triggerRef)}
          />
        )
      : undefined;
  const selecting = canWrite
    ? { selection: bulk, rowName: (member: Member) => member.fullName }
    : {};

  return (
    <section
      ref={card}
      aria-labelledby="members-heading"
      className="bg-card shadow-card overflow-hidden rounded-xl border"
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
          name={(member) => member.fullName}
          actions={[
            {
              label: t("classDetail.removeSelected"),
              description: t("classDetail.removeSelectedBody"),
              icon: UserMinus,
              run: (member) => removeMember(klass.id, member.userId),
            },
          ]}
          onRemoved={bulk.remove}
          onClear={bulk.clear}
          onSettled={() => invalidateClassMembership(queryClient, klass.id)}
        />
      ) : null}

      {members.isPending ? (
        <MembersSkeleton />
      ) : (
        <DataTable
          label={t("classDetail.membersHeading", { count: klass.studentCount })}
          columns={columns}
          rows={rows}
          rowSize={{ minHeight: 56 }}
          shown={MEMBER_SETS[band] ?? ALL_COLUMNS}
          {...selecting}
          rowTone={(member) => (bulk.selected.has(member.id) ? "selected" : undefined)}
          menu={menu}
          menuLabel={(member) =>
            t("classDetail.memberActions", { name: member.fullName })
          }
          menuTrack="32px"
          padX={18}
          framed={false}
          empty={members.isError ? null : emptyLine(t, search, joinOff)}
        />
      )}

      <MembersFooter members={members} />

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
        description={t(
          joinOff
            ? "classDetail.removeConfirmBodyOff"
            : "classDetail.removeConfirmBody",
        )}
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

function MembersFooter({ members }: Readonly<{ members: UseQueryResult<MemberPage> }>) {
  const { t } = useTranslation();
  const data = members.data;
  return (
    <>
      {members.isError ? (
        <div className="border-t px-4.5 py-4">
          <LoadError error={members.error} onRetry={() => void members.refetch()}>
            {t("classDetail.membersFailed")}
          </LoadError>
        </div>
      ) : null}
      {data !== undefined && data.total > PAGE_SIZES[0] ? (
        <Pager
          page={data.page}
          pageSize={data.pageSize}
          total={data.total}
          noun={(count) => t("classDetail.memberNoun", { count })}
          sizes={PAGE_SIZES}
        />
      ) : null}
      <p className="text-muted-fg border-t px-4.5 py-2.25 text-xs">
        {t("classDetail.removeKeepsWork")}
      </p>
    </>
  );
}

function emptyLine(t: Translate, search: string, joinOff: boolean): string {
  if (search === "")
    return t(joinOff ? "classDetail.noMembersOff" : "classDetail.noMembers");
  return t("classDetail.noMemberMatches", { query: search });
}

function memberColumns(t: Translate, locale: Locale): readonly DataColumn<Member>[] {
  return [
    {
      id: "name",
      header: t("classDetail.name"),
      track: "minmax(180px,2fr)",
      cell: (member) => (
        <span className="flex min-w-0 items-center gap-2.5">
          <Avatar name={member.fullName} size="30" />
          <span className="min-w-0">
            <span className="block truncate font-medium">{member.fullName}</span>
            <span className="text-muted-fg block truncate text-xs leading-normal">
              {member.email}
            </span>
          </span>
        </span>
      ),
    },
    {
      id: "via",
      header: t("classDetail.joinedVia"),
      track: "120px",
      cell: (member) => <JoinedVia member={member} />,
    },
    {
      id: "at",
      header: t("classDetail.joinedAt"),
      track: "80px",
      cell: (member) => (
        <span className="text-muted-fg block text-sm leading-normal whitespace-nowrap">
          {joinedOn(member.joinedAt, locale)}
        </span>
      ),
    },
    {
      id: "submitted",
      header: t("students.submitted"),
      track: "80px",
      align: "end",
      cell: (member) => (
        <span className="tabular-nums">{member.stats.submittedCount}</span>
      ),
    },
    {
      id: "average",
      header: t("students.average"),
      track: "70px",
      align: "end",
      cell: (member) => <Average member={member} />,
    },
  ];
}

function JoinedVia({ member }: Readonly<{ member: Member }>) {
  const { t } = useTranslation();
  const byTeacher = member.joinedVia === "admin";
  return (
    <span
      className={cn(
        "inline-flex h-5.5 items-center rounded-sm px-2 text-xs font-medium whitespace-nowrap",
        byTeacher ? "bg-primary text-primary-fg" : "bg-muted font-mono",
      )}
    >
      {byTeacher
        ? t("classDetail.viaAdmin")
        : t("classDetail.viaCode", { hint: member.joinCodeHint ?? "" })}
    </span>
  );
}

function Average({ member }: Readonly<{ member: Member }>) {
  const { t } = useTranslation();
  const percent = scorePercent(member.stats);
  if (percent === null) return <span className="text-muted-fg">—</span>;
  return (
    <span
      className={cn(
        "font-semibold tabular-nums",
        percent < LOW_AVERAGE && "text-danger-ink",
      )}
    >
      {t("students.percent", { value: percent })}
    </span>
  );
}

function MemberMenuItems({
  member,
  canWrite,
  canSeeResults,
  onRemove,
}: Readonly<{
  member: Member;
  canWrite: boolean;
  canSeeResults: boolean;
  onRemove: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <>
      {canSeeResults ? (
        <DropdownMenuItem asChild>
          <Link to={`/teacher/students?studentId=${member.userId}`}>
            <ChartColumn aria-hidden="true" />
            {t("classDetail.viewResults")}
          </Link>
        </DropdownMenuItem>
      ) : null}
      {canWrite && canSeeResults ? <DropdownMenuSeparator /> : null}
      {canWrite ? (
        <DropdownMenuItem variant="destructive" onSelect={onRemove}>
          <UserMinus aria-hidden="true" />
          {t("classDetail.remove")}
        </DropdownMenuItem>
      ) : null}
    </>
  );
}

function joinedOn(at: string, locale: Locale): string {
  const zone = getDisplayTimeZone();
  const year = (moment: string | Date) => formatInTimeZone(moment, zone, "yyyy");
  return year(at) === year(new Date())
    ? dayMonth(at, locale)
    : dayMonthYear(at, locale);
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
