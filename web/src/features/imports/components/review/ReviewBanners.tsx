import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import type { WordImport } from "../../api";
import { isActiveStatus } from "../../status";
import { ReprocessNotice } from "../ReprocessNotice";

const BAR = "flex shrink-0 flex-wrap items-center gap-3 border-b px-4 py-2";

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
  committedTestId,
  reprocessed,
  onReload,
  onReloadStale,
  onAdopt,
}: Readonly<{
  value: WordImport;
  stale: boolean;
  finished: boolean;
  committedTestId: string | null;
  reprocessed: boolean;
  onReload: () => void;
  onReloadStale: () => void;
  onAdopt: () => void;
}>) {
  const { t } = useTranslation();
  const testId = committedTestId ?? value.testId ?? null;
  const committed = value.status === "committed" || committedTestId !== null;
  return (
    <>
      {stale ? (
        <div role="alert" className={BAR}>
          <p className="text-sm">{t("imports.review.staleBody")}</p>
          <Button
            variant="outline"
            size="sm"
            className="ml-auto"
            onClick={onReloadStale}
          >
            {t("imports.review.reload")}
          </Button>
        </div>
      ) : null}
      {finished ? (
        <div role="status" className={BAR}>
          <p className="text-sm">{t("imports.review.finishedBody")}</p>
          <Button variant="outline" size="sm" className="ml-auto" onClick={onReload}>
            {t("imports.review.reload")}
          </Button>
        </div>
      ) : null}
      {committed ? (
        <div role="status" className={BAR}>
          <p className="text-sm">{t("imports.review.committedBanner")}</p>
          {testId === null ? null : (
            <Button asChild variant="outline" size="sm" className="ml-auto">
              <Link to={`/admin/tests/${testId}/edit`}>
                {t("imports.detail.openBuilder")}
              </Link>
            </Button>
          )}
        </div>
      ) : null}
      {!committed && value.status !== "needs_review" ? (
        <div role="status" className={BAR}>
          <p className="text-sm">{t(`imports.review.readOnly.${value.status}`)}</p>
          <Button asChild variant="outline" size="sm" className="ml-auto">
            <Link to={`/admin/imports/${value.id}`}>
              {isActiveStatus(value.status)
                ? t("imports.review.viewProgress")
                : t("imports.review.viewImport")}
            </Link>
          </Button>
        </div>
      ) : null}
      <ReprocessNotice value={value} className="shrink-0 border-b px-4 py-2 text-sm" />
      {reprocessed ? (
        <div role="status" className={BAR}>
          <p className="text-sm">{t("imports.review.reprocessed")}</p>
          <Button variant="outline" size="sm" className="ml-auto" onClick={onAdopt}>
            {t("imports.review.adopt")}
          </Button>
        </div>
      ) : null}
    </>
  );
}

/** PhoneNotice stands in for the review below 768px: the open counts and where to continue. */
export function PhoneNotice({
  title,
  importId,
  blocking,
  review,
}: Readonly<{ title: string; importId: string; blocking: number; review: number }>) {
  const { t } = useTranslation();
  return (
    <div className="space-y-3 rounded-lg border p-5" role="status">
      <h1 className="text-lg font-semibold break-words">{title}</h1>
      <p className="text-sm">{t("imports.review.phoneCounts", { blocking, review })}</p>
      <p className="text-muted-foreground text-sm">
        {t("imports.review.phoneGuidance")}
      </p>
      <Button asChild variant="outline" size="sm">
        <Link to={`/admin/imports/${importId}`}>{t("imports.review.back")}</Link>
      </Button>
    </div>
  );
}
