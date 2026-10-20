import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  BookOpenText,
  Download,
  ExternalLink,
  LoaderCircle,
  ShieldCheck,
} from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { CopyButton } from "@/components/shared/CopyButton";
import { QueryStates } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { notify } from "@/lib/toast";
import { BASE_URL } from "@/lib/api/client";
import { failureMessage } from "@/lib/api/errors";
import { PageHead } from "@/layouts/shell/PageHead";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import {
  apiReferenceQuery,
  downloadOpenApi,
  type ApiReferenceSummary,
} from "../../api";
import { API_GROUPS, FALLBACK_GROUP } from "../../groups";
import { useDocsLauncher } from "../../useDocsLauncher";

const BUTTON = "h-9 gap-1.5 whitespace-nowrap";

/**
 * ApiReferencePage is the Admin console's API reference screen: the
 * contract's version and endpoint count, a button that opens the interactive
 * reference on the API origin in a new tab (a group's card opens it at that
 * group), and the OpenAPI document as a download.
 */
export default function ApiReferencePage() {
  const { t } = useTranslation();
  const summary = useQuery(apiReferenceQuery());
  const docs = useDocsLauncher();
  return (
    <div className="flex w-full min-w-0 flex-col gap-4.5">
      <PageHead title={t("apiReference.title")} description={t("apiReference.lead")} />
      <Hero summary={summary.data} loading={summary.isPending} docs={docs} />
      <section aria-labelledby="api-reference-inside" className="flex flex-col gap-2.5">
        <div className="flex flex-wrap items-baseline justify-between gap-2.5">
          <h2 id="api-reference-inside" className="text-md font-semibold">
            {t("apiReference.inside")}
          </h2>
          <span className="text-muted-fg text-meta">
            {t("apiReference.insideHint")}
          </span>
        </div>
        <QueryStates
          query={summary}
          skeleton={<GroupsSkeleton />}
          failed={t("apiReference.loadFailed")}
        >
          {(data) => <Groups groups={data.groups} docs={docs} />}
        </QueryStates>
      </section>
    </div>
  );
}

function Hero({
  summary,
  loading,
  docs,
}: Readonly<{
  summary: ApiReferenceSummary | undefined;
  loading: boolean;
  docs: ReturnType<typeof useDocsLauncher>;
}>) {
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const download = () => {
    if (saving) return;
    setSaving(true);
    downloadOpenApi()
      .catch((cause: unknown) => {
        notify.error(failureMessage(cause, t("apiReference.downloadFailed")));
      })
      .finally(() => setSaving(false));
  };
  const Open = docs.pending ? LoaderCircle : ExternalLink;
  return (
    <section
      aria-label={t("apiReference.hero")}
      className="bg-card shadow-card flex flex-col gap-4 rounded-xl border p-5"
    >
      <div className="flex flex-wrap items-start gap-3.5">
        <span
          aria-hidden="true"
          className="bg-primary text-primary-fg grid size-11 shrink-0 place-items-center rounded-[0.6875rem]"
        >
          <BookOpenText className="size-5" />
        </span>
        <div className="flex min-w-0 flex-[1_1_16.25rem] flex-col gap-1">
          <span className="text-base font-semibold">{t("apiReference.hero")}</span>
          {loading ? <Skeleton className="h-4 w-56" /> : null}
          {summary === undefined ? null : (
            <span className="text-muted-fg text-meta flex flex-wrap gap-2">
              {t("apiReference.version", {
                openapi: summary.openapi,
                version: summary.version,
                count: summary.endpointCount,
              })}
            </span>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            className={BUTTON}
            aria-disabled={saving || undefined}
            onClick={download}
          >
            <Download aria-hidden="true" />
            {t("apiReference.download")}
          </Button>
          <Button
            type="button"
            className={cn(BUTTON, "font-semibold")}
            aria-disabled={docs.pending || undefined}
            onClick={() => docs.open()}
          >
            <Open
              aria-hidden="true"
              className={docs.pending ? "animate-spin" : undefined}
            />
            {docs.pending ? t("apiReference.opening") : t("apiReference.open")}
          </Button>
        </div>
      </div>
      <div className="bg-muted flex flex-wrap items-center gap-2.5 rounded-[0.5625rem] px-3 py-2.5">
        <span className="text-muted-fg text-meta">{t("apiReference.baseUrl")}</span>
        <code className="text-ui min-w-0 flex-[1_1_12.5rem] truncate font-mono">
          {BASE_URL}
        </code>
        <CopyButton
          value={BASE_URL}
          failedMessage={t("apiReference.copyFailed")}
          className="bg-card border px-2.5"
        />
      </div>
      {docs.problem === null ? null : (
        <Callout
          tone="danger"
          size="md"
          announce
          actions={
            <button
              type="button"
              className="text-ui font-semibold whitespace-nowrap underline underline-offset-2"
              onClick={docs.retry}
            >
              {t("common.retry")}
            </button>
          }
        >
          {t(`apiReference.${docs.problem}`)}
        </Callout>
      )}
      <p className="text-muted-fg text-meta flex gap-2.5 leading-[1.55]">
        <ShieldCheck aria-hidden="true" className="mt-px size-[0.9375rem] shrink-0" />
        <span>{t("apiReference.security")}</span>
      </p>
    </section>
  );
}

function Groups({
  groups,
  docs,
}: Readonly<{
  groups: ApiReferenceSummary["groups"];
  docs: ReturnType<typeof useDocsLauncher>;
}>) {
  const { t, i18n } = useTranslation();
  const locale = useLocale();
  const count = new Intl.NumberFormat(locale);
  return (
    <ul className="grid list-none grid-cols-[repeat(auto-fill,minmax(min(100%,18.75rem),1fr))] gap-2.5 p-0">
      {groups.map(({ tag, count: endpoints }) => {
        const known = API_GROUPS[tag];
        const { icon: Icon, path } = known ?? FALLBACK_GROUP;
        const named = i18n.exists(`apiReference.groups.${tag}.name`);
        const name = named ? t(`apiReference.groups.${tag}.name`) : tag;
        return (
          <li key={tag} className="flex">
            <button
              type="button"
              aria-disabled={docs.pending || undefined}
              onClick={() => docs.open(tag)}
              className="bg-card shadow-card hover:border-ring flex w-full min-w-0 items-start gap-3 rounded-xl border px-4 py-3.5 text-left"
            >
              <span
                aria-hidden="true"
                className="bg-muted grid size-8.5 shrink-0 place-items-center rounded-[0.5625rem]"
              >
                <Icon className="size-4" />
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="flex items-center justify-between gap-2">
                  <span className="text-ui font-semibold">{name}</span>
                  <span className="text-muted-fg text-xs whitespace-nowrap tabular-nums">
                    {t("apiReference.endpoints", {
                      count: endpoints,
                      n: count.format(endpoints),
                    })}
                  </span>
                </span>
                {named ? (
                  <span className="text-muted-fg text-meta leading-normal text-pretty">
                    {t(`apiReference.groups.${tag}.blurb`)}
                  </span>
                ) : null}
                {path === "" ? null : (
                  <code className="text-muted-fg truncate font-mono text-[0.71875rem]">
                    {path}
                  </code>
                )}
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function GroupsSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-label={t("common.loading")}
      className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,18.75rem),1fr))] gap-2.5"
    >
      {Array.from({ length: 6 }, (_, index) => (
        <Skeleton key={index} className="h-24 rounded-xl" />
      ))}
    </div>
  );
}
