import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
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
import { listTests, listVersions, type TestVersion } from "@/features/tests/api";

export interface PickedVersion {
  testId: string;
  testTitle: string;
  version: TestVersion;
}

/**
 * G-01's card 1. Only published tests appear, because only a published version
 * can be assigned and offering a draft would produce a 409 after the teacher
 * had filled in the rest of the form.
 */
export function TestVersionPicker({
  open,
  onOpenChange,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (picked: PickedVersion) => void;
}) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState<string | null>(null);

  const tests = useLazyList({
    queryKey: ["admin-tests", "picker", { status: "published" }],
    fetchPage: (page, signal) =>
      listTests({ status: "published", page, limit: 20 }, signal),
    enabled: open,
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("assignments.pickTest")}</DialogTitle>
          <DialogDescription>{t("assignments.pickTestHint")}</DialogDescription>
        </DialogHeader>

        <div className="max-h-96 space-y-1 overflow-y-auto">
          {tests.isPending ? (
            <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
          ) : tests.isError ? (
            <p role="alert" className="text-destructive text-sm">
              {t("tests.loadFailed")}
            </p>
          ) : tests.items.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              {t("assignments.noPublishedTests")}
            </p>
          ) : (
            tests.items.map((test) => (
              <div key={test.id} className="rounded-md border">
                <button
                  type="button"
                  aria-expanded={expanded === test.id}
                  className="hover:bg-secondary/50 flex w-full items-center gap-3 rounded-md p-3 text-left"
                  onClick={() => setExpanded(expanded === test.id ? null : test.id)}
                >
                  <FileText
                    className="text-muted-foreground size-5 shrink-0"
                    aria-hidden="true"
                  />
                  {/* Title and latest version only. `questionCount` and
                    `totalPoints` on a Test describe its DRAFT outline, and this
                    screen is choosing a frozen version -- a test whose draft was
                    emptied after publishing would advertise "0 câu" for a
                    version that has plenty. The numbers live on the rows below,
                    where they are the version's own. */}
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{test.title}</p>
                    <p className="text-muted-foreground text-xs">
                      {t("assignments.latestVersion", {
                        version: test.currentVersion,
                      })}
                    </p>
                  </div>
                </button>

                {expanded === test.id ? (
                  <VersionList
                    testId={test.id}
                    onPick={(version) => {
                      onPick({ testId: test.id, testTitle: test.title, version });
                      onOpenChange(false);
                    }}
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

function VersionList({
  testId,
  onPick,
}: {
  testId: string;
  onPick: (version: TestVersion) => void;
}) {
  const { t } = useTranslation();
  const versions = useQuery({
    queryKey: ["admin-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId, signal),
  });

  if (versions.isPending) {
    return (
      <p className="text-muted-foreground px-3 pb-3 text-xs">{t("common.loading")}</p>
    );
  }
  if (versions.isError) {
    return (
      <p role="alert" className="text-destructive px-3 pb-3 text-xs">
        {t("tests.loadFailed")}
      </p>
    );
  }

  return (
    <ul className="space-y-1 border-t p-2">
      {versions.data?.items.map((version) => (
        <li key={version.id}>
          <button
            type="button"
            className="hover:bg-secondary flex w-full items-center gap-3 rounded-sm px-2 py-1.5 text-left text-xs"
            onClick={() => onPick(version)}
          >
            <span className="font-medium tabular-nums">
              {t("tests.versionNumber", { n: version.version })}
            </span>
            <span className="text-muted-foreground">
              {t("assignments.versionMeta", {
                questions: version.questionCount,
                points: version.totalPoints,
                audio: version.audioCount,
              })}
            </span>
            <span className="text-muted-foreground ml-auto">
              {t("assignments.use")}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}
