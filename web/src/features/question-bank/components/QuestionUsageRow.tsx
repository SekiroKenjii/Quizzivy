import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { getQuestion } from "@/features/question-bank/api";

/** QuestionUsageRow loads editable test references when a bank row is expanded. */
export function QuestionUsageRow({
  questionId,
  prompt,
}: Readonly<{ questionId: string; prompt: string }>) {
  const { t } = useTranslation();
  const usage = useQuery({
    queryKey: ["admin-question-usage", questionId],
    queryFn: ({ signal }) => getQuestion(questionId, signal),
    staleTime: 0,
  });
  const tests = usage.data?.usedIn ?? [];
  return (
    <TableRow className="hover:bg-transparent">
      <TableCell colSpan={8} className="bg-muted/30 p-4">
        <section
          id={`question-usage-${questionId}`}
          aria-label={t("bank.usageTitle", { prompt })}
          className="space-y-2"
        >
          <p className="text-muted-foreground text-xs">{t("bank.usageHint")}</p>
          {usage.isPending ? (
            <p role="status" className="text-sm">
              {t("common.loading")}
            </p>
          ) : null}
          {usage.isError ? (
            <div role="alert" className="flex items-center gap-2 text-sm">
              <span>{t("bank.usageFailed")}</span>
              <Button variant="outline" size="sm" onClick={() => void usage.refetch()}>
                {t("common.retry")}
              </Button>
            </div>
          ) : null}
          {usage.isSuccess && tests.length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("bank.usageEmpty")}</p>
          ) : null}
          {usage.isSuccess && tests.length > 0 ? (
            <div className="bg-background max-h-64 overflow-auto rounded-md border">
              <Table aria-label={t("bank.usageTitle", { prompt })}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("tests.title")}</TableHead>
                    <TableHead className="w-48">{t("bank.usageLocation")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tests.map((test) => (
                    <TableRow key={test.id}>
                      <TableCell>
                        <Link
                          className="font-medium break-words hover:underline"
                          to={`/admin/tests/${test.id}/edit`}
                        >
                          {test.title}
                        </Link>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {t("bank.currentOutline")}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : null}
        </section>
      </TableCell>
    </TableRow>
  );
}
