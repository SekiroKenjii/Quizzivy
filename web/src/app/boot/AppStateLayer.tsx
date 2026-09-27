import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import type { DataRouter } from "react-router";
import { ArrowRight, Sparkles } from "lucide-react";
import { queryClient } from "@/app/queryClient";
import { MaintenancePage } from "@/app/pages/MaintenancePage";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useAppState, type MaintenanceWindow } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { isActive } from "./activity";
import { readStatus } from "./status";
import { SplashFrame, SplashText } from "./SplashFrame";

const RECHECK_MS = 60_000;

function useInertOutside(host: HTMLElement | null) {
  useEffect(() => {
    if (host === null) return;
    const made = new Set<Element>();
    const quiet = (element: Element) => {
      if (element === host || element.hasAttribute("inert")) return;
      element.setAttribute("inert", "");
      made.add(element);
    };
    for (const child of Array.from(document.body.children)) quiet(child);
    const observer = new MutationObserver((records) => {
      for (const record of records) {
        for (const node of Array.from(record.addedNodes)) {
          if (node instanceof Element) quiet(node);
        }
      }
    });
    observer.observe(document.body, { childList: true });
    return () => {
      observer.disconnect();
      for (const element of made) element.removeAttribute("inert");
    };
  }, [host]);
}

function useHost(active: boolean): HTMLElement | null {
  const [host] = useState(() => {
    const element = document.createElement("div");
    element.dataset["appState"] = "";
    return element;
  });
  useLayoutEffect(() => {
    if (!active) return;
    document.body.append(host);
    return () => host.remove();
  }, [active, host]);
  return active ? host : null;
}

/**
 * AppStateLayer shows the overlay that stands over the page -- maintenance, a
 * session that needs signing in again, or a newer build -- above everything,
 * with the rest of the document inert and focus moved into it, while the page
 * underneath stays mounted.
 *
 * Maintenance closes when `GET /public/status` reports no window under way,
 * asked by "Check again" and every minute while the user is active; closing
 * refetches every query and, if the session was never restored, restores it.
 * "Sign in" drops the session but not the answer drafts, so the session guard
 * sends the user to sign in and back to this page, which resumes from them.
 */
export function AppStateLayer({ router }: Readonly<{ router: DataRouter }>) {
  const { t } = useTranslation();
  const overlay = useAppState((s) => s.overlay);
  const host = useHost(overlay.kind !== "none");
  useInertOutside(host);

  let label = t("splash.updateTitle");
  let content: ReactNode = <UpdateCard />;
  if (overlay.kind === "maintenance") {
    label = t("maintenance.title");
    content = <MaintenanceOverlay window={overlay.window} />;
  } else if (overlay.kind === "expired") {
    label = t("splash.expiredTitle");
    content = <ExpiredCard router={router} />;
  }

  if (overlay.kind === "none" || host === null) return null;
  return createPortal(<FocusOnOpen label={label}>{content}</FocusOnOpen>, host);
}

function FocusOnOpen({
  label,
  children,
}: Readonly<{ label: string; children: ReactNode }>) {
  const container = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const target = container.current?.querySelector<HTMLElement>("button, a[href]");
    (target ?? container.current)?.focus();
  }, [label]);
  return (
    <div
      ref={container}
      role="dialog"
      aria-modal="true"
      aria-label={label}
      tabIndex={-1}
      className="bg-bg fixed inset-0 z-(--z-app-state) overflow-y-auto outline-none"
    >
      {children}
    </div>
  );
}

function MaintenanceOverlay({
  window: current,
}: Readonly<{ window: MaintenanceWindow }>) {
  const check = useCallback(async () => {
    let status: Awaited<ReturnType<typeof readStatus>>;
    try {
      status = await readStatus();
    } catch {
      return;
    }
    const state = useAppState.getState();
    if (status.kind === "maintenance") {
      state.showOverlay(status);
      return;
    }
    state.closeOverlay();
    if (useAuthStore.getState().expired) state.showOverlay({ kind: "expired" });
    if (state.bootPhase !== "ready") state.retryBoot();
    void queryClient.invalidateQueries();
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (isActive()) void check();
    }, RECHECK_MS);
    return () => window.clearInterval(timer);
  }, [check]);

  return <MaintenancePage window={current} onCheck={() => void check()} />;
}

function ExpiredCard({ router }: Readonly<{ router: DataRouter }>) {
  const { t } = useTranslation();
  const signIn = () => {
    const { pathname, search } = router.state.location;
    useAuthStore.getState().clearSession();
    queryClient.clear();
    useAppState.getState().closeOverlay();
    window.setTimeout(() => {
      const now = router.state.location;
      if (now.pathname !== pathname || now.search !== search) return;
      void router.navigate(`/login?next=${encodeURIComponent(pathname + search)}`, {
        replace: true,
      });
    }, 0);
  };
  return (
    <SplashFrame mark="still">
      <div className="qz-boot-state">
        <SplashText title={t("splash.expiredTitle")} body={t("splash.expiredBody")} />
        <Button size="md" className="px-4 font-semibold" onClick={signIn}>
          {t("splash.signIn")}
          <ArrowRight aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
    </SplashFrame>
  );
}

function UpdateCard() {
  const { t } = useTranslation();
  return (
    <SplashFrame mark="still">
      <div className="qz-boot-state">
        <Badge variant="info" className="text-meta border-0 in-data-[scale=deck]:h-6.5">
          <Sparkles aria-hidden="true" className="size-[13px]" />
          {t("splash.updatePill")}
        </Badge>
        <SplashText title={t("splash.updateTitle")} body={t("splash.updateBody")} />
        <Button
          size="md"
          className="px-4 font-semibold"
          onClick={() => window.location.reload()}
        >
          {t("splash.updateNow")}
        </Button>
      </div>
    </SplashFrame>
  );
}
