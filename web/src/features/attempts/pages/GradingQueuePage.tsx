import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Flag } from "lucide-react";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { Pager } from "@/components/shared/Pager";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { usePage } from "@/hooks/usePage";
import { useLocale } from "@/lib/i18n/useLocale";
import { formatRelative } from "@/lib/i18n/datetime";
import { listAttempts, type AttemptListRow } from "../api";

type Tab = "pending" | "flagged";
const PAGE_SIZE = 20;

/**
 * G-10: the teacher's inbox. "Chờ chấm" is a nav item rather than a dashboard
 * widget (A-00) because it is the one queue that has to be emptied, and it
 * used to be reachable only through a monitor screen.
 */
export default function GradingQueuePage() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const tab: Tab = params.get("tab") === "flagged" ? "flagged" : "pending";
  const locale = useLocale();

  const [page] = usePage(tab);
  const attempts = useQuery({
    queryKey: ["admin-attempts", { tab, page }],
    queryFn: ({ signal }) =>
      listAttempts(
        {
          limit: PAGE_SIZE,
          page,
          ...(tab === "pending" ? { pendingGrading: true } : { flagged: true }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });
  const items = attempts.data?.items ?? [];

  return (
    <div className="space-y-4">
      <PageHeader
        variant="title"
        title={t("nav.grading")}
        subtitle={
          attempts.isSuccess
            ? t(`queue.summary.${tab}`, { count: attempts.data.total })
            : " "
        }
      />

      <Tabs
        value={tab}
        onValueChange={(next) => {
          setParams((current) => {
            const out = new URLSearchParams(current);
            out.delete("page");
            if (next === "flagged") out.set("tab", "flagged");
            else out.delete("tab");
            return out;
          });
        }}
      >
        <TabsList aria-label={t("queue.filter")}>
          <TabsTrigger value="pending">{t("queue.tabs.pending")}</TabsTrigger>
          <TabsTrigger value="flagged">{t("queue.tabs.flagged")}</TabsTrigger>
        </TabsList>
      </Tabs>

      <QueryStates
        query={attempts}
        skeleton={<ListSkeleton />}
        failed={t("queue.loadFailed")}
      >
        {(data) =>
          items.length === 0 ? (
            <EmptyState
              action={
                <Button size="sm" variant="outline" asChild>
                  <Link to="/admin/assignments">{t("queue.toAssignments")}</Link>
                </Button>
              }
            >
              {t(`queue.empty.${tab}`)}
            </EmptyState>
          ) : (
            <>
              <Card className="gap-0 overflow-hidden py-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[28%]">
                        {t("queue.columns.student")}
                      </TableHead>
                      <TableHead>{t("queue.columns.test")}</TableHead>
                      <TableHead>{t("queue.columns.state")}</TableHead>
                      <TableHead>{t("queue.columns.submittedAt")}</TableHead>
                      <TableHead className="text-right">
                        {t("queue.columns.pending")}
                      </TableHead>
                      <TableHead className="w-28" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.map((row) => (
                      <Row key={row.id} row={row} locale={locale} />
                    ))}
                  </TableBody>
                </Table>
              </Card>
              <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
            </>
          )
        }
      </QueryStates>
    </div>
  );
}

function Row({ row, locale }: Readonly<{ row: AttemptListRow; locale: "vi" | "en" }>) {
  const { t } = useTranslation();
  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-2">
          <Avatar name={row.studentName} size="sm" />
          <span className="font-medium">{row.studentName}</span>
        </div>
      </TableCell>
      <TableCell className="text-muted-foreground">
        <Link
          to={`/admin/assignments/${row.assignmentId}`}
          className="hover:text-foreground"
        >
          {row.testTitle}
        </Link>
      </TableCell>
      <TableCell>
        <span className="inline-flex items-center gap-1.5">
          <StatusBadge kind="attempt" status={row.status} />
          {row.flagged && (
            <Badge variant="warning">
              <Flag aria-hidden="true" />
              {t("status.attention.flagged")}
            </Badge>
          )}
        </span>
      </TableCell>
      <TableCell className="text-muted-foreground whitespace-nowrap">
        {row.submittedAt ? formatRelative(row.submittedAt, locale) : "—"}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {row.pendingManual ?? 0}
      </TableCell>
      <TableCell className="text-right">
        <Button size="sm" variant="outline" asChild>
          <Link to={`/admin/attempts/${row.id}`}>{t("queue.open")}</Link>
        </Button>
      </TableCell>
    </TableRow>
  );
}
