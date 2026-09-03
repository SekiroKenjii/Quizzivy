import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { isRouteErrorResponse, useLocation, useRouteError } from "react-router";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ErrorActions, ErrorScreen } from "@/app/pages/ErrorScreen";
import { NotFoundArt, UnexpectedErrorArt } from "@/app/pages/errorArt";

/**
 * The global error boundary from §9: "global error boundary with reload +
 * copyable error ID".
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
    <ErrorScreen
      art={<UnexpectedErrorArt />}
      title={t("error.title")}
      body={t("error.body")}
      footer={t("error.footnote")}
    >
      <ErrorActions>
        <Button onClick={() => window.location.reload()}>{t("error.reload")}</Button>
        <Button variant="outline" asChild>
          <a href="/">{t("error.home")}</a>
        </Button>
      </ErrorActions>

      <Separator className="my-5" />
      <div className="flex items-center justify-center gap-2">
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
  const location = useLocation();

  return (
    <ErrorScreen
      art={<NotFoundArt />}
      title={t("notFound.title")}
      body={t("notFound.body")}
      footer={t("notFound.footnote")}
    >
      {/* The path that failed, on screen. */}
      <div className="bg-muted/50 mt-4 rounded-md px-3 py-2">
        <code className="text-muted-foreground font-mono text-xs break-words">
          {location.pathname}
        </code>
      </div>

      <ErrorActions>
        <Button asChild>
          <a href="/">{t("notFound.action")}</a>
        </Button>
        <Button variant="outline" onClick={() => window.history.back()}>
          {t("notFound.back")}
        </Button>
      </ErrorActions>
    </ErrorScreen>
  );
}
