import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { RadioGroup } from "radix-ui";
import type { TFunction } from "i18next";
import { Search } from "lucide-react";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { Button } from "@/components/ui/button";
import type { PickedVersion } from "@/features/assignments/components/TestVersionPicker";
import {
  listTests,
  listVersions,
  type Test,
  type TestVersion,
} from "@/features/tests/api";
import { useLazyList } from "@/hooks/useLazyList";
import { useDebounced } from "@/lib/useDebounced";

const PAGE = 20;

/**
 * TestStep is the wizard's "Which test?": the teacher's published tests as
 * radio rows, searched by title, each picking its current version.
 */
export function TestStep({
  picked,
  onPick,
}: Readonly<{
  picked: PickedVersion | null;
  onPick: (picked: PickedVersion) => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const client = useQueryClient();
  const [query, setQuery] = useState("");
  const search = useDebounced(query.trim(), 250);
  const queryKey = ["admin-tests", "wizard", { q: search }];
  const tests = useLazyList({
    queryKey,
    fetchPage: (page, signal) =>
      listTests(
        search === ""
          ? { status: "published", page, limit: PAGE }
          : { status: "published", q: search, page, limit: PAGE },
        signal,
      ),
  });

  function choose(testId: string) {
    const test = tests.items.find((item) => item.id === testId);
    const versions = client.getQueryData<{ items: TestVersion[] }>([
      "admin-test-versions",
      testId,
    ]);
    const version = versions?.items.find(
      (item) => item.version === test?.currentVersion,
    );
    if (test && version) onPick({ testId: test.id, testTitle: test.title, version });
  }

  return (
    <div className="flex flex-col gap-3">
      <h2 id={titleId} className="text-md font-semibold">
        {t("assignments.wizard.testTitle")}
      </h2>
      <label className="border-border bg-bg focus-within:border-ring flex h-9 items-center gap-2 rounded-lg border px-2.5">
        <Search aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("assignments.wizard.searchTests")}
          aria-label={t("assignments.wizard.searchTests")}
          className="text-ui min-w-0 flex-1 bg-transparent outline-none"
        />
      </label>
      {tests.isPending && <ListSkeleton rows={4} />}
      {tests.isError && (
        <LoadError
          error={null}
          onRetry={() => void client.refetchQueries({ queryKey })}
        >
          {t("assignments.wizard.testsFailed")}
        </LoadError>
      )}
      {!tests.isPending && !tests.isError && tests.items.length === 0 && (
        <p className="text-muted-fg rounded-[10px] border border-dashed px-3.5 py-6 text-center text-sm">
          {search === ""
            ? t("assignments.noPublishedTests")
            : t("assignments.wizard.testsNoMatch", { query: search })}
        </p>
      )}
      {tests.items.length > 0 && (
        <RadioGroup.Root
          aria-labelledby={titleId}
          value={picked?.testId ?? ""}
          onValueChange={choose}
          className="flex flex-col gap-3"
        >
          {tests.items.map((test) => (
            <TestRow key={test.id} test={test} />
          ))}
          <LoadMoreSentinel
            active={tests.hasMore}
            loading={tests.loadingMore}
            onVisible={tests.loadMore}
          />
        </RadioGroup.Root>
      )}
    </div>
  );
}

function TestRow({ test }: Readonly<{ test: Test }>) {
  const { t } = useTranslation();
  const versions = useQuery({
    queryKey: ["admin-test-versions", test.id],
    queryFn: ({ signal }) => listVersions(test.id, signal),
  });
  const current = versions.data?.items.find(
    (version) => version.version === test.currentVersion,
  );
  if (versions.isError) {
    return (
      <div className="border-border flex flex-wrap items-center gap-3 rounded-[10px] border px-3.5 py-3">
        <span className="min-w-0 flex-[1_1_200px]">
          <span className="block text-base leading-[1.3] font-medium [overflow-wrap:anywhere]">
            {test.title}
          </span>
          <span role="alert" className="text-danger text-meta block leading-[1.3]">
            {t("assignments.wizard.versionsFailed")}
          </span>
        </span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => void versions.refetch()}
        >
          {t("common.retry")}
        </Button>
      </div>
    );
  }
  const meta = current === undefined ? "" : versionMeta(current, t);
  return (
    <RadioGroup.Item
      value={test.id}
      disabled={current === undefined}
      className="group bg-card border-border data-[state=checked]:border-primary data-[state=checked]:bg-muted flex items-center gap-3 rounded-[10px] border px-3.5 py-3 text-left disabled:cursor-progress"
    >
      <span
        aria-hidden="true"
        className="border-ring group-data-[state=checked]:border-primary grid size-4.5 flex-none place-items-center rounded-full border-2"
      >
        <span className="group-data-[state=checked]:bg-primary size-2 rounded-full" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-base leading-[1.3] font-medium [overflow-wrap:anywhere]">
          {test.title}
        </span>
        {meta === "" ? null : (
          <span className="text-muted-fg text-meta block leading-[1.3]">{meta}</span>
        )}
      </span>
    </RadioGroup.Item>
  );
}

function versionMeta(version: TestVersion, t: TFunction): string {
  return [
    t("assignments.wizard.questions", { count: version.questionCount }),
    version.manualCount > 0
      ? t("assignments.wizard.manual", { count: version.manualCount })
      : "",
  ]
    .filter((part) => part !== "")
    .join(" · ");
}
