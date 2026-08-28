import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { isRouteErrorResponse, useRouteError } from "react-router";
import { Button } from "@/components/ui/button";

/**
 * The global error boundary from §9: "global error boundary with reload +
 * copyable error ID".
 *
 * The copyable ID is the point. A student who hits an error can read a short
 * code to their teacher, and it matches the `requestId` the API returns in its
 * error envelope (docs/plan/00-overview.md §7), so a report can be traced to a
 * specific request rather than a vague description.
 *
 * Tone follows §12: plain text, no alarm iconography.
 */
function useErrorId(error: unknown): string {
  const [id] = useState(() => {
    // Prefer the server's requestId so client and server agree on the label.
    if (error && typeof error === "object" && "requestId" in error) {
      const rid = (error as { requestId?: unknown }).requestId;
      if (typeof rid === "string" && rid) return rid;
    }
    return crypto.randomUUID();
  });
  return id;
}

export function ErrorBoundary() {
  const error = useRouteError();
  const { t } = useTranslation();
  const errorId = useErrorId(error);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    // Surfaced for the operator; a real reporter arrives with observability.
    console.error("[quizzivy] unhandled route error", { errorId, error });
  }, [error, errorId]);

  // A 404 from the router is not an application failure; render the real page.
  if (isRouteErrorResponse(error) && error.status === 404) {
    return <NotFound />;
  }

  async function copyId() {
    try {
      await navigator.clipboard.writeText(errorId);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center gap-6 p-6">
      <div className="space-y-2">
        <h1 className="text-xl font-semibold tracking-tight">{t("error.title")}</h1>
        <p className="text-muted-foreground text-sm leading-relaxed">{t("error.body")}</p>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Button onClick={() => window.location.reload()}>{t("error.reload")}</Button>
        <Button variant="outline" onClick={copyId} aria-label={t("error.copyId")}>
          {copied ? t("error.copied") : t("error.copyId")}
        </Button>
      </div>

      <p className="text-muted-foreground font-mono text-xs">
        {t("error.errorId")}
        {": "}
        <span className="select-all">{errorId}</span>
      </p>
    </main>
  );
}

export function NotFound() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center gap-6 p-6">
      <div className="space-y-2">
        <h1 className="text-xl font-semibold tracking-tight">{t("notFound.title")}</h1>
        <p className="text-muted-foreground text-sm leading-relaxed">{t("notFound.body")}</p>
      </div>
      <div>
        <Button asChild>
          <a href="/">{t("notFound.action")}</a>
        </Button>
      </div>
    </main>
  );
}
