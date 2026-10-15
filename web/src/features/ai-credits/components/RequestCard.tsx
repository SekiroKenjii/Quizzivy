import { useTranslation } from "react-i18next";
import { Check, Hand } from "lucide-react";
import { IconTile } from "@/components/shared/IconTile";
import { Button } from "@/components/ui/button";
import type { AiPendingRequest } from "@/features/ai-credits/api";
import { useDecideRequest } from "@/features/ai-credits/mutations";
import { credits } from "@/features/ai/credits";
import { formatAgo } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";

/**
 * RequestCard is one request for more credits, in warning tones: who asked
 * for how much and how much of their allowance they have used, their reason
 * and how long ago they asked, and Decline and Approve. While the request is
 * being decided both buttons answer `aria-disabled` and ignore a press, so
 * the one that was pressed keeps focus. `onDecided` is called once the card
 * has left the list, so the page can put focus where it was.
 */
export function RequestCard({
  request,
  onDecided,
}: Readonly<{ request: AiPendingRequest; onDecided: () => void }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const decide = useDecideRequest(request, onDecided);
  const busy = decide.isPending;
  const amount = credits(request.amount, locale);
  function press(decision: "approve" | "decline") {
    if (!busy) decide.mutate(decision);
  }
  return (
    <li className="bg-card shadow-card flex flex-wrap items-center gap-3 rounded-xl border py-3 pr-3.5 pl-4">
      <IconTile icon={Hand} size={32} tone="warning" />
      <div className="min-w-0 flex-[1_1_17.5rem]">
        <p className="text-ui leading-normal font-medium">
          {t("aiCredits.request.title", {
            name: request.teacherName,
            amount,
            used: credits(request.used, locale),
            allowance: credits(request.allowance, locale),
          })}
        </p>
        <p className="text-muted-fg text-meta leading-normal text-pretty break-words">
          {t("aiCredits.request.why", {
            reason: request.reason,
            ago: formatAgo(request.createdAt, locale, t("common.agoJustNow")),
          })}
        </p>
      </div>
      <div className="flex flex-wrap gap-1.5">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-disabled={busy || undefined}
          className="hover:bg-muted aria-disabled:text-foreground/50 h-8 px-3 aria-disabled:cursor-not-allowed aria-disabled:hover:bg-transparent"
          onClick={() => press("decline")}
        >
          {t("aiCredits.request.decline")}
        </Button>
        <Button
          type="button"
          size="sm"
          aria-disabled={busy || undefined}
          className="aria-disabled:bg-primary/50 aria-disabled:hover:bg-primary/50 h-8 px-3 aria-disabled:cursor-not-allowed"
          onClick={() => press("approve")}
        >
          <Check aria-hidden="true" />
          {t("aiCredits.request.approve", { amount })}
        </Button>
      </div>
    </li>
  );
}
