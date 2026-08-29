import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { isRouteErrorResponse, useRouteError } from "react-router";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ErrorScreen } from "@/app/pages/ErrorScreen";

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
    // The deck's shape: short enough to read aloud, in two groups.
    const bytes = crypto.getRandomValues(new Uint8Array(4));
    const hex = [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 4)}-${hex.slice(4)}`;
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
    <ErrorScreen title={t("error.title")} body={t("error.body")}>
      <Button className="mt-5" onClick={() => window.location.reload()}>
        {t("error.reload")}
      </Button>

      <div className="mt-6 flex items-center justify-center gap-2">
        <span className="text-muted-foreground text-xs">{t("error.errorId")}</span>
        <code className="rounded-sm border px-1.5 py-0.5 font-mono text-xs select-all">
          {errorId}
        </code>
        <Button variant="ghost" size="xs" onClick={() => void copyId()}>
          <Copy aria-hidden="true" />
          {copied ? t("error.copied") : t("error.copyId")}
        </Button>
      </div>
    </ErrorScreen>
  );
}

export function NotFound() {
  const { t } = useTranslation();
  return (
    <ErrorScreen title={t("notFound.title")} body={t("notFound.body")}>
      <Button asChild className="mt-5">
        <a href="/">{t("notFound.action")}</a>
      </Button>
    </ErrorScreen>
  );
}
