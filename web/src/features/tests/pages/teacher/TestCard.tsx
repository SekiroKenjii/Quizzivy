import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  Archive,
  Award,
  Copy,
  Ellipsis,
  ListChecks,
  RotateCw,
  Send,
  Trash2,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { BulkSelectRow, type BulkSelection } from "@/components/shared/BulkSelection";
import { StatusBadge } from "@/components/shared/StatusBadge";
import type { Test, TestStatus } from "@/features/tests/api";
import { assignedLabel } from "@/features/tests/testFacts";
import { formatRelative } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

/**
 * TestCardActions are what a card's controls do to its test. The menu's
 * trigger carries `data-test-menu` with the test's id, so a dialog opened
 * from the menu can give focus back to it.
 */
export type TestCardActions = Readonly<{
  onDuplicate: (test: Test) => void;
  onArchive: (test: Test) => void;
  onRestore: (test: Test) => void;
  onDelete: (test: Test) => void;
}>;

const PILL_SHAPE = "border-0 in-data-[scale=deck]:px-2";

const PILL: Record<TestStatus, string> = {
  published: `${PILL_SHAPE} [&>[aria-hidden]]:bg-success`,
  draft: `${PILL_SHAPE} bg-transparent text-muted-fg [&>[aria-hidden]]:bg-border`,
  archived: `${PILL_SHAPE} bg-muted text-muted-fg in-data-[scale=deck]:rounded-full [&>[aria-hidden]]:bg-muted-fg`,
};

const ICON_BUTTON =
  "bg-card text-fg hover:bg-hover data-[state=open]:bg-hover rounded-seg grid size-6.5 flex-none cursor-pointer place-items-center border disabled:pointer-events-none disabled:opacity-45";

/**
 * TestCard is one test of the Tests list, as the deck draws it: the status
 * and when it was last edited, the title, the skills its questions practise,
 * and a footer with its questions, points and assignments. The whole card
 * opens the test's detail. Above that link sit the selection checkbox, the
 * Duplicate button and the "…" menu (Duplicate and Archive, with Assign to a
 * class first for a published test; Restore, Duplicate and Delete for an
 * archived one). `duplicated` marks a test just
 * duplicated or just made by duplicating; `duplicating` disables Duplicate.
 */
export function TestCard({
  test,
  selection,
  duplicated,
  duplicating,
  ...actions
}: Readonly<
  {
    test: Test;
    selection: BulkSelection<Test>;
    duplicated: boolean;
    duplicating: boolean;
  } & TestCardActions
>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <article
      data-test-card={test.id}
      className="bg-card shadow-card hover:border-ring relative flex min-w-0 flex-col overflow-hidden rounded-xl border transition-colors duration-150"
    >
      <div className="flex flex-1 flex-col gap-2.5 px-4 pt-4 pb-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="relative z-10 inline-flex">
              <BulkSelectRow item={test} name={test.title} selection={selection} />
            </span>
            <StatusBadge
              kind="test"
              status={test.status}
              dot
              className={PILL[test.status]}
            />
            {duplicated ? (
              <Badge variant="brand" className={PILL_SHAPE}>
                {t("common.justDuplicated")}
              </Badge>
            ) : null}
          </div>
          <span className="text-muted-fg shrink-0 text-xs leading-normal">
            {t("tests.edited", { when: formatRelative(test.updatedAt, locale) })}
          </span>
        </div>
        <h2 className="text-body leading-[1.35] font-semibold [overflow-wrap:anywhere]">
          <Link
            to={`/teacher/tests/${test.id}`}
            className="after:absolute after:inset-0 after:rounded-xl"
          >
            {test.title}
          </Link>
        </h2>
        {test.skills.length > 0 ? (
          <ul
            aria-label={t("tests.skills")}
            className="m-0 flex list-none flex-wrap gap-1.5 p-0"
          >
            {test.skills.map((skill) => (
              <li
                key={skill}
                className="bg-muted text-fg inline-flex h-5.5 items-center rounded-full border px-2 text-xs leading-none font-medium whitespace-nowrap"
              >
                {t(`tests.skill.${skill}`)}
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      <div className="bg-sidebar text-meta text-muted-fg flex flex-wrap items-center gap-x-3 gap-y-2 border-t px-4 py-2.5 leading-normal">
        <p className="m-0 flex min-w-0 flex-[1_1_auto] flex-wrap items-center gap-x-3.5 gap-y-1">
          <Fact icon={ListChecks}>
            {t("tests.questionCount", { count: test.questionCount })}
          </Fact>
          <Fact icon={Award}>
            {t("tests.pointCount", {
              count: test.totalPoints,
              points: new Intl.NumberFormat(locale).format(test.totalPoints),
            })}
          </Fact>
          <Fact icon={Send}>{assignedLabel(test, t)}</Fact>
        </p>
        <div className="relative z-10 flex flex-none items-center gap-1.5">
          <button
            type="button"
            aria-label={t("common.duplicateNamed", { name: test.title })}
            disabled={duplicating}
            onClick={() => actions.onDuplicate(test)}
            className={ICON_BUTTON}
          >
            <Copy aria-hidden="true" className="size-3.25" />
          </button>
          <CardMenu test={test} {...actions} />
        </div>
      </div>
    </article>
  );
}

function Fact({
  icon: Icon,
  children,
}: Readonly<{ icon: LucideIcon; children: string }>) {
  return (
    <span className="flex items-center gap-1.25 whitespace-nowrap">
      <Icon aria-hidden="true" className="size-3.25 flex-none" />
      {children}
    </span>
  );
}

function CardMenu({
  test,
  onDuplicate,
  onArchive,
  onRestore,
  onDelete,
}: Readonly<{ test: Test } & TestCardActions>) {
  const { t } = useTranslation();
  const archived = test.status === "archived";
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          data-test-menu={test.id}
          aria-label={t("tests.actionsFor", { title: test.title })}
          className={ICON_BUTTON}
        >
          <Ellipsis aria-hidden="true" className="size-3.75" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52 data-[scale=deck]:w-55">
        {archived ? (
          <DropdownMenuItem onSelect={() => onRestore(test)}>
            <RotateCw aria-hidden="true" />
            {t("tests.restore")}
          </DropdownMenuItem>
        ) : null}
        {test.status === "published" ? (
          <DropdownMenuItem asChild>
            <Link to={`/teacher/assignments/new?test=${test.id}`}>
              <Send aria-hidden="true" />
              {t("tests.assignToClass")}
            </Link>
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem onSelect={() => onDuplicate(test)}>
          <Copy aria-hidden="true" />
          {t("tests.duplicate")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {archived ? (
          <DropdownMenuItem variant="destructive" onSelect={() => onDelete(test)}>
            <Trash2 aria-hidden="true" />
            {t("common.deletePermanently")}
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem variant="destructive" onSelect={() => onArchive(test)}>
            <Archive aria-hidden="true" />
            {t("tests.archive")}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
