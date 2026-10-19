import { useRef, useState, type ReactNode, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { LoadError } from "@/components/shared/ListState";
import { StickyAside } from "@/components/shared/StickyAside";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  adminCreditsQuery,
  type AiAdminCredits,
  type AiTeacherMonth,
} from "@/features/ai-credits/api";
import { AddCreditsDialog } from "@/features/ai-credits/components/AddCreditsDialog";
import { AllowanceDialog } from "@/features/ai-credits/components/AllowanceDialog";
import { PoolCard } from "@/features/ai-credits/components/PoolCard";
import { RequestCard } from "@/features/ai-credits/components/RequestCard";
import { RulesCard } from "@/features/ai-credits/components/RulesCard";
import { TeachersCard } from "@/features/ai-credits/components/TeachersCard";
import { isAiOff } from "@/features/ai/api";
import { resetsOn } from "@/features/ai/credits";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { PageHead } from "@/layouts/shell/PageHead";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

const DOCKED_FROM = 900;

function CardSkeleton({
  className,
  rows = 0,
}: Readonly<{ className?: string; rows?: number }>) {
  return (
    <div className={cn("bg-card shadow-card rounded-xl border", className)}>
      <div className="flex flex-col gap-1.5 border-b px-4.5 py-4">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-3.5 w-64 max-w-full" />
      </div>
      <div className="flex flex-col gap-4 p-4.5">
        {Array.from({ length: rows }, (_, row) => (
          <Skeleton key={row} className="h-9 w-full" />
        ))}
      </div>
    </div>
  );
}

function CreditsSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="flex flex-col gap-4"
    >
      <CardSkeleton rows={3} />
      <div className="flex flex-wrap items-start gap-4">
        <CardSkeleton className="min-w-0 flex-[2_1_35rem]" rows={5} />
        <CardSkeleton className="min-w-0 flex-[1_1_20rem]" rows={4} />
      </div>
    </div>
  );
}

function Credits({
  view,
  headingRef,
}: Readonly<{
  view: AiAdminCredits;
  headingRef: RefObject<HTMLHeadingElement | null>;
}>) {
  const { t } = useTranslation();
  const docked = useContentWidthAtLeast(DOCKED_FROM);
  const [editing, setEditing] = useState<AiTeacherMonth | null>(null);
  const editor = useRef<HTMLElement | null>(null);
  return (
    <>
      <PoolCard view={view} />
      {view.requests.length > 0 && (
        <ul aria-label={t("aiCredits.request.list")} className="flex flex-col gap-4">
          {view.requests.map((request) => (
            <RequestCard
              key={request.id}
              request={request}
              onDecided={() => headingRef.current?.focus()}
            />
          ))}
        </ul>
      )}
      <div className="flex flex-wrap items-start gap-4">
        <TeachersCard
          view={view}
          onEdit={(teacher, trigger) => {
            editor.current = trigger;
            setEditing(teacher);
          }}
        />
        <StickyAside docked={docked} className="min-w-0 flex-[1_1_20rem]">
          <RulesCard rules={view.rules} />
        </StickyAside>
      </div>
      <AllowanceDialog
        teacher={editing}
        defaultAllowance={view.rules.defaultAllowance}
        onClose={() => setEditing(null)}
        returnFocus={editor}
      />
    </>
  );
}

/**
 * AiCreditsPage is the Admin console's AI credits (T-R5.35): the centre's pool
 * for the month, the requests for more credits waiting for an Admin, the
 * teachers with their month and the rules that apply to every teacher
 * without a custom allowance, all from `GET /admin/ai/credits`. "Add credits"
 * is offered once the month has loaded. While it loads each card is a
 * skeleton; when it cannot be read the page says so with Retry; on a server
 * without an assistant (501) it says so in one sentence and draws no card. A
 * read that fails after the first one leaves what was drawn in place.
 */
export default function AiCreditsPage() {
  const { t } = useTranslation();
  const locale = useLocale();
  const query = useQuery(adminCreditsQuery());
  const [adding, setAdding] = useState(false);
  const addButton = useRef<HTMLButtonElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const view = query.data;
  const month = view?.month;

  let body: ReactNode;
  if (view !== undefined) body = <Credits view={view} headingRef={heading} />;
  else if (query.isPending) body = <CreditsSkeleton />;
  else if (isAiOff(query.error))
    body = <p className="text-muted-fg text-base">{t("aiCredits.off")}</p>;
  else
    body = (
      <LoadError error={query.error} onRetry={() => void query.refetch()}>
        {t("aiCredits.loadFailed")}
      </LoadError>
    );

  return (
    <div className="flex flex-col gap-4">
      <PageHead
        title={t("aiCredits.title")}
        description={
          month === undefined
            ? t("aiCredits.lead")
            : t("aiCredits.leadReset", { date: resetsOn(month, locale) })
        }
        headingRef={heading}
        actions={
          view === undefined ? undefined : (
            <Button
              ref={addButton}
              type="button"
              variant="outline"
              onClick={() => setAdding(true)}
            >
              <Plus aria-hidden="true" />
              {t("aiCredits.add.open")}
            </Button>
          )
        }
      />
      {body}
      <AddCreditsDialog
        open={adding}
        onOpenChange={setAdding}
        returnFocus={addButton}
      />
    </div>
  );
}
