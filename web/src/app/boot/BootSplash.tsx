import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";
import type { DataRouter } from "react-router";
import { RotateCw, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { skeletonFor } from "./handoff";
import { SplashFrame, SplashText } from "./SplashFrame";

const SLOW_AFTER_MS = 8_000;
const FADE_MS = 350;
const RETRY_AFTER_S = 10;
const FILL = ["12%", "46%", "78%", "100%"] as const;
const STEP_KEYS = [
  "splash.steps.session",
  "splash.steps.classes",
  "splash.steps.ready",
] as const;

type Leave = "showing" | "leaving" | "gone";

function useRouterSettled(router: DataRouter): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => router.subscribe(() => onChange()),
    [router],
  );
  return useSyncExternalStore(
    subscribe,
    () => router.state.initialized && router.state.navigation.state === "idle",
  );
}

/**
 * BootSplash covers the app while it starts: restoring the session, then
 * loading the first route's code, then handing over. The bar moves only when a
 * step is done, with no minimum duration. A step that takes eight seconds
 * turns it into the slow state; a session restore that got no answer turns it
 * into the offline state, which retries on its own after ten seconds and as
 * soon as the browser is back online. It gives way to any overlay, and fades
 * out when boot fails so the error page shows.
 */
export function BootSplash({ router }: Readonly<{ router: DataRouter }>) {
  const bootstrapping = useAuthStore((s) => s.isBootstrapping);
  const bootPhase = useAppState((s) => s.bootPhase);
  const overlaid = useAppState((s) => s.overlay.kind !== "none");
  const settled = useRouterSettled(router);
  const [step, setStep] = useState(0);
  const [leave, setLeave] = useState<Leave>("showing");
  const attempt = useAppState((s) => s.bootAttempt);
  const [slowAt, setSlowAt] = useState<string | null>(null);

  let target = 2;
  if (bootstrapping) target = 0;
  else if (!settled) target = 1;
  if (target > step && leave === "showing") setStep(target);

  const finished = step >= 2 && !bootstrapping && settled;
  const failed = bootPhase === "failed";

  useEffect(() => {
    if (leave !== "showing" || !(finished || failed)) return;
    let second = 0;
    const first = requestAnimationFrame(() => {
      second = requestAnimationFrame(() => {
        if (!failed && router.state.navigation.state !== "idle") return;
        setStep(3);
        setLeave("leaving");
      });
    });
    return () => {
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
    };
  }, [leave, finished, failed, router]);

  useEffect(() => {
    if (leave !== "leaving") return;
    const timer = window.setTimeout(() => setLeave("gone"), FADE_MS);
    return () => window.clearTimeout(timer);
  }, [leave]);

  const waiting = leave === "showing" && bootPhase === "booting";
  const moment = `${attempt}:${step}`;
  const slow = waiting && slowAt === moment;

  useEffect(() => {
    if (!waiting) return;
    const timer = window.setTimeout(() => setSlowAt(moment), SLOW_AFTER_MS);
    return () => window.clearTimeout(timer);
  }, [moment, waiting]);

  if (leave === "gone" || overlaid) return null;

  if (bootPhase === "offline") {
    return (
      <SplashFrame mark="offline">
        <OfflineState />
      </SplashFrame>
    );
  }

  const skeleton =
    leave === "leaving" ? skeletonFor(router.state.location.pathname) : null;
  return (
    <>
      {skeleton !== null && (
        <div aria-hidden="true" className="fixed inset-0 z-(--z-splash)">
          {skeleton}
        </div>
      )}
      <SplashFrame mark="busy" leaving={leave === "leaving"}>
        {slow ? <SlowState /> : <LoadingState step={step} />}
      </SplashFrame>
    </>
  );
}

function LoadingState({ step }: Readonly<{ step: number }>) {
  const { t } = useTranslation();
  return (
    <div className="qz-boot-state">
      <div className="qz-boot-track">
        <div className="qz-boot-fill" style={{ width: FILL[step] }} />
      </div>
      <span className="qz-boot-label">{t(STEP_KEYS[Math.min(step, 2)]!)}</span>
    </div>
  );
}

function SlowState() {
  const { t } = useTranslation();
  return (
    <div className="qz-boot-state">
      <div className="qz-boot-track">
        <span className="qz-indet bg-fg absolute inset-y-0 left-0 w-2/5 rounded-[3px]" />
      </div>
      <SplashText
        title={t("splash.slowTitle")}
        body={t("splash.slowBody")}
        size="body"
      />
      <Button
        variant="outline"
        className="rounded-ctl py-0"
        onClick={() => window.location.reload()}
      >
        <RotateCw aria-hidden="true" className="size-3.5" />
        {t("splash.reload")}
      </Button>
    </div>
  );
}

function OfflineState() {
  const { t } = useTranslation();
  const retryBoot = useAppState((s) => s.retryBoot);
  const [seconds, setSeconds] = useState(RETRY_AFTER_S);

  useEffect(() => {
    const timer = window.setInterval(() => {
      setSeconds((left) => Math.max(left - 1, 0));
    }, 1_000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (seconds === 0) retryBoot();
  }, [seconds, retryBoot]);

  useEffect(() => {
    window.addEventListener("online", retryBoot);
    return () => window.removeEventListener("online", retryBoot);
  }, [retryBoot]);

  return (
    <div className="qz-boot-state">
      <Badge
        variant="warning"
        className="text-meta border-0 in-data-[scale=deck]:h-6.5"
      >
        <WifiOff aria-hidden="true" className="size-[13px]" />
        {t("splash.offlinePill")}
      </Badge>
      <SplashText title={t("splash.offlineTitle")} body={t("splash.offlineBody")} />
      <div className="flex gap-2">
        <Button size="md" className="px-4 font-semibold" onClick={retryBoot}>
          <RotateCw aria-hidden="true" className="size-3.5" />
          {t("splash.tryAgain")}
        </Button>
      </div>
      <p className="text-muted-fg text-xs leading-normal">
        {seconds > 0 ? t("splash.retryIn", { seconds }) : t("splash.retrying")}
      </p>
    </div>
  );
}
