import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useLazyList } from "@/hooks/useLazyList";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { FileText } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { getTest, listTests, saveOutline } from "@/features/tests/api";
import { ApiError } from "@/lib/api/errors";

/** A-06's "Thêm vào đề thi" for a selection. */
export function AddToTestDialog({
  questionIds,
  open,
  onOpenChange,
  onAdded,
}: {
  questionIds: string[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const tests = useLazyList({
    queryKey: ["admin-tests", "picker", { status: "all" }],
    fetchPage: (page, signal) => listTests({ page, limit: 20 }, signal),
    enabled: open,
  });

  const add = useMutation({
    mutationFn: async ({
      testId,
      sectionId,
    }: {
      testId: string;
      sectionId: string;
    }) => {
      const test = await getTest(testId);
      return saveOutline(testId, {
        expectedUpdatedAt: test.updatedAt,
        title: test.title,
        description: test.description ?? null,
        sections: test.sections.map((section) => ({
          id: section.id,
          title: section.title,
          instructions: section.instructions ?? null,
          questionIds:
            section.id === sectionId
              ? // Already-present ids are dropped: adding a question a section
                // holds twice is not something the builder can represent.
                [...new Set([...section.questionIds, ...questionIds])]
              : section.questionIds,
        })),
      });
    },
    onSuccess: async () => {
      setError(null);
      await queryClient.invalidateQueries({ queryKey: ["admin-tests"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      onAdded();
      onOpenChange(false);
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("bank.addToTestFailed")),
  });

  const candidates = tests.items.filter((x) => x.status !== "archived");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("bank.addToTest")}</DialogTitle>
          <DialogDescription>
            {t("bank.addToTestHint", { count: questionIds.length })}
          </DialogDescription>
        </DialogHeader>

        {error === null ? null : (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        )}

        <div className="max-h-96 space-y-1 overflow-y-auto">
          {tests.isPending ? (
            <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
          ) : candidates.length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("bank.noTestsToAddTo")}</p>
          ) : (
            candidates.map((test) => (
              <div key={test.id} className="rounded-md border">
                <button
                  type="button"
                  aria-expanded={expanded === test.id}
                  className="hover:bg-secondary/50 flex w-full items-center gap-3 rounded-md p-3 text-left"
                  onClick={() => setExpanded(expanded === test.id ? null : test.id)}
                >
                  <FileText
                    className="text-muted-foreground size-4 shrink-0"
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">
                    {test.title}
                  </span>
                  <span className="text-muted-foreground text-xs">
                    {t(`builder.${test.status}`)}
                  </span>
                </button>

                {expanded === test.id ? (
                  <SectionList
                    testId={test.id}
                    pending={add.isPending}
                    onPick={(sectionId) => add.mutate({ testId: test.id, sectionId })}
                  />
                ) : null}
              </div>
            ))
          )}
          <LoadMoreSentinel
            active={tests.hasMore}
            loading={tests.loadingMore}
            onVisible={tests.loadMore}
          />
        </div>
      </DialogContent>
    </Dialog>
  );
}

function SectionList({
  testId,
  pending,
  onPick,
}: {
  testId: string;
  pending: boolean;
  onPick: (sectionId: string) => void;
}) {
  const { t } = useTranslation();
  const test = useQuery({
    queryKey: ["admin-test", testId],
    queryFn: ({ signal }) => getTest(testId, signal),
  });

  if (test.isPending) {
    return (
      <p className="text-muted-foreground px-3 pb-3 text-xs">{t("common.loading")}</p>
    );
  }
  if (test.isError) {
    return (
      <p role="alert" className="text-destructive px-3 pb-3 text-xs">
        {t("tests.loadFailed")}
      </p>
    );
  }
  if (test.data.sections.length === 0) {
    return (
      <p className="text-muted-foreground px-3 pb-3 text-xs">{t("bank.noSections")}</p>
    );
  }

  return (
    <ul className="space-y-1 border-t p-2">
      {test.data.sections.map((section) => (
        <li key={section.id}>
          <button
            type="button"
            disabled={pending}
            className="hover:bg-secondary flex w-full items-center gap-3 rounded-sm px-2 py-1.5 text-left text-xs disabled:opacity-50"
            onClick={() => onPick(section.id)}
          >
            <span className="truncate">{section.title}</span>
            <span className="text-muted-foreground ml-auto">
              {t("bank.addHere", { count: section.questionIds.length })}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}
