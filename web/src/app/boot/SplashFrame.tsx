import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { appVersion } from "./version";

/**
 * SplashFrame is the deck's splash: the 64px mark over one state block, the
 * version at the foot. It uses index.html's `qz-boot` classes, so its first
 * frame is the static splash the browser painted before the bundle ran. `mark`
 * breathes while `busy` and greys out when `offline`; `leaving` fades it out
 * and takes it out of the way.
 */
export function SplashFrame({
  mark,
  leaving = false,
  children,
}: Readonly<{
  mark: "busy" | "still" | "offline";
  leaving?: boolean;
  children: ReactNode;
}>) {
  const { t } = useTranslation();
  return (
    <div
      data-scale="deck"
      role="status"
      aria-live="polite"
      aria-hidden={leaving || undefined}
      className={cn("qz-boot", leaving && "pointer-events-none opacity-0")}
    >
      <div className="qz-boot-column">
        <span
          className={cn(
            "qz-boot-mark",
            mark === "busy" && "qz-breath",
            mark === "offline" && "opacity-60 grayscale",
          )}
        >
          <img
            className="qz-boot-light"
            src="/brand/quizzivy-mark-color.svg"
            width={64}
            height={64}
            alt={t("app.name")}
            draggable={false}
          />
          <img
            className="qz-boot-dark"
            src="/brand/quizzivy-mark-on-dark.svg"
            width={64}
            height={64}
            alt={t("app.name")}
            draggable={false}
          />
        </span>
        {children}
      </div>
      <span className="qz-boot-version">
        {t("splash.version", { version: appVersion })}
      </span>
    </div>
  );
}

/** SplashText is a state's title and body, as the deck sets them. */
export function SplashText({
  title,
  body,
  size = "md",
}: Readonly<{ title: string; body: string; size?: "md" | "body" }>) {
  return (
    <div className="flex flex-col gap-1">
      <p
        className={cn(
          size === "md" ? "text-md" : "text-body",
          "leading-normal font-semibold",
        )}
      >
        {title}
      </p>
      <p className="text-muted-fg text-sm leading-[1.55] text-pretty">{body}</p>
    </div>
  );
}
