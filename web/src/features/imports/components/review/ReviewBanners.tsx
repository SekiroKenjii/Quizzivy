import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Monitor, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { WordImport } from "../../api";
import { isActiveStatus } from "../../status";
import { ReprocessNotice } from "../ReprocessNotice";

const BAR = "flex flex-none flex-wrap items-center gap-3 border-b px-4 py-2";
const BAR_BUTTON = "ml-auto h-8 rounded-lg px-3 text-sm shadow-none";

/**
 * ReviewBanners are the review's state notices: a stale draft, processing that
 * finished after the page opened, the draft test already created, a review
 * that is read-only while the import is elsewhere in its lifecycle, a newer
 * processing result that can be adopted, and a reprocess that did not finish.
 * `onReloadStale` asks before discarding unsaved edits; `onReload` does not.
 */
export function ReviewBanners({
  value,
  stale,
  finished,
  reprocessed,
  onReload,
  onReloadStale,
  onAdopt,
}: Readonly<{
  value: WordImport;
  stale: boolean;
  finished: boolean;
  reprocessed: boolean;
  onReload: () => void;
  onReloadStale: () => void;
  onAdopt: () => void;
}>) {
  const { t } = useTranslation();
  const testId = value.testId ?? null;
  const committed = value.status === "committed";
  return (
    <>
      {stale ? (
        <div role="alert" className={`${BAR} bg-danger-soft`}>
          <p className="m-0 text-sm">{t("imports.review.staleBody")}</p>
          <Button variant="outline" className={BAR_BUTTON} onClick={onReloadStale}>
            {t("imports.review.reload")}
          </Button>
        </div>
      ) : null}
      {finished ? (
        <div role="status" className={`${BAR} bg-info-soft`}>
          <p className="m-0 text-sm">{t("imports.review.finishedBody")}</p>
          <Button variant="outline" className={BAR_BUTTON} onClick={onReload}>
            {t("imports.review.reload")}
          </Button>
        </div>
      ) : null}
      {committed ? (
        <div role="status" className={`${BAR} bg-success-soft`}>
          <p className="m-0 text-sm">{t("imports.review.committedBanner")}</p>
          {testId === null ? null : (
            <Button asChild variant="outline" className={BAR_BUTTON}>
              <Link to={`/teacher/tests/${testId}/edit`}>
                {t("imports.detail.openBuilder")}
              </Link>
            </Button>
          )}
        </div>
      ) : null}
      {!committed && value.status !== "needs_review" ? (
        <div role="status" className={`${BAR} bg-muted`}>
          <p className="m-0 text-sm">{t(`imports.review.readOnly.${value.status}`)}</p>
          <Button asChild variant="outline" className={BAR_BUTTON}>
            <Link to={`/teacher/imports/${value.id}`}>
              {isActiveStatus(value.status)
                ? t("imports.review.viewProgress")
                : t("imports.review.viewImport")}
            </Link>
          </Button>
        </div>
      ) : null}
      <ReprocessNotice
        value={value}
        className="m-0 flex-none border-b px-4 py-2 text-sm"
      />
      {reprocessed ? (
        <div role="status" className={`${BAR} bg-info-soft`}>
          <p className="m-0 text-sm">{t("imports.review.reprocessed")}</p>
          <Button variant="outline" className={BAR_BUTTON} onClick={onAdopt}>
            {t("imports.review.adopt")}
          </Button>
        </div>
      ) : null}
    </>
  );
}

/**
 * PhoneNote is the deck's dismissible band on a phone: the review is read
 * here, and edited on a larger screen.
 */
export function PhoneNote({ onDismiss }: Readonly<{ onDismiss: () => void }>) {
  const { t } = useTranslation();
  return (
    <div
      role="note"
      className="bg-info-soft text-info-ink text-meta flex flex-none items-start gap-2.5 px-4 py-2.5 leading-normal"
    >
      <Monitor aria-hidden="true" className="mt-px size-3.75 flex-none" />
      <span className="min-w-0 flex-1">{t("imports.review.phoneNote")}</span>
      <button
        type="button"
        aria-label={t("display.dismiss")}
        onClick={onDismiss}
        className="grid size-6 flex-none cursor-pointer place-items-center rounded-md"
      >
        <X aria-hidden="true" className="size-3.5" />
      </button>
    </div>
  );
}
