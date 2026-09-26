import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { isRouteErrorResponse, useRouteError } from "react-router";
import { ErrorActions, ErrorScreen } from "@/app/pages/ErrorScreen";
import { MaintenancePage } from "@/app/pages/MaintenancePage";
import NotFoundPage from "@/app/pages/NotFoundPage";
import { UnexpectedErrorArt } from "@/app/pages/errorArt";
import { CopyButton } from "@/components/shared/CopyButton";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { homePathFor } from "@/features/auth/home";
import { maintenanceWindow } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

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

/**
 * ErrorBoundary is §9's global error boundary. A router 404 renders the 404
 * page and a 503 `MAINTENANCE` the maintenance page; anything else is the
 * unexpected-error page, with the server's request id as the error ID when
 * the error carries one.
 */
export function ErrorBoundary() {
  const error = useRouteError();
  const { t } = useTranslation();
  const errorId = useErrorId(error);
  const user = useAuthStore((s) => s.user);

  useEffect(() => {
    console.error("[quizzivy] unhandled route error", { errorId, error });
  }, [error, errorId]);

  if (isRouteErrorResponse(error) && error.status === 404) {
    return <NotFoundPage />;
  }
  const maintenance = maintenanceWindow(error);
  if (maintenance) {
    return (
      <MaintenancePage window={maintenance} onCheck={() => window.location.reload()} />
    );
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
          <a href={user ? homePathFor(user) : "/login"}>{t("error.home")}</a>
        </Button>
      </ErrorActions>

      <Separator className="my-5" />
      <div className="flex flex-wrap items-center justify-center gap-2">
        <span className="text-muted-fg text-xs">{t("error.errorId")}</span>
        <code className="rounded-[4px] border px-1.5 py-0.5 font-mono text-xs select-all">
          {errorId}
        </code>
        <CopyButton value={errorId} failedMessage={t("error.copyFailed")} />
      </div>
    </ErrorScreen>
  );
}
