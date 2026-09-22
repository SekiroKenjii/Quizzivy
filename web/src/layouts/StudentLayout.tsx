import { useMemo, useState } from "react";
import { NavLink, Outlet, useMatches, useNavigate } from "react-router";
import { ArrowLeft, User } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { BrandLockup, BrandMark } from "@/components/shared/Brand";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import type { DetailShell } from "@/layouts/detailShell";

interface StudentHandle {
  detail?: boolean;
  titleKey?: string;
}

/** StudentLayout keeps the route mounted while adapting navigation at 1024px. */
export default function StudentLayout() {
  const { t } = useTranslation();
  const wide = useMediaQuery("(min-width: 1024px)");
  const matches = useMatches();
  const handle = [...matches].reverse().find((m) => isStudentHandle(m.handle))
    ?.handle as StudentHandle | undefined;
  const [own, setTitle] = useState<string | null>(null);
  const context = useMemo(() => ({ setTitle }) satisfies DetailShell, []);

  return (
    <div className="student-surface flex min-h-svh flex-col">
      <header className="bg-background border-b">
        {!wide && handle?.detail ? (
          <DetailBar
            title={own ?? (handle.titleKey ? t(handle.titleKey) : t("app.name"))}
          />
        ) : (
          <div className="flex min-h-14 items-center gap-3 px-4 lg:px-8">
            {wide ? (
              <BrandLockup height={28} />
            ) : (
              <BrandMark height={24} label={false} />
            )}
            <nav
              aria-label={t("nav.mainNavigation")}
              className="ml-auto flex items-center gap-1 lg:ml-4"
            >
              <NavLink to="/app" end className={link}>
                {t("student.myAssignments")}
              </NavLink>
              <NavLink to="/app/classes" className={link}>
                {t("student.classesNav")}
              </NavLink>
              {!wide && (
                <NavLink
                  to="/app/settings"
                  className="text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex size-11 items-center justify-center rounded-md focus-visible:ring-2"
                  aria-label={t("nav.settings")}
                >
                  <User className="size-5" aria-hidden="true" />
                </NavLink>
              )}
            </nav>
            {wide && (
              <div className="ml-auto">
                <AccountMenu settingsTo="/app/settings" named />
              </div>
            )}
          </div>
        )}
      </header>
      <main
        className="w-full min-w-0 flex-1 p-4 lg:p-8"
        style={{ paddingBottom: "max(2rem, env(safe-area-inset-bottom))" }}
      >
        <Outlet context={context} />
      </main>
    </div>
  );
}

const link = ({ isActive }: { isActive: boolean }) =>
  cn(
    "focus-visible:ring-ring inline-flex min-h-11 items-center rounded-md px-3 text-sm whitespace-nowrap transition-colors focus-visible:ring-2 lg:min-h-8",
    isActive
      ? "bg-secondary text-secondary-foreground"
      : "text-muted-foreground hover:text-foreground",
  );

function DetailBar({ title }: Readonly<{ title: string }>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <div className="flex min-h-14 w-full items-center gap-2 px-4">
      <Button
        variant="ghost"
        size="icon"
        aria-label={t("common.back")}
        onClick={() => void (cameFromInside() ? navigate(-1) : navigate("/app"))}
      >
        <ArrowLeft aria-hidden="true" />
      </Button>
      <p className="min-w-0 truncate text-sm font-medium">{title}</p>
    </div>
  );
}

function cameFromInside(): boolean {
  const state = window.history.state as { idx?: number } | null;
  return typeof state?.idx === "number" && state.idx > 0;
}

function isStudentHandle(handle: unknown): handle is StudentHandle {
  return typeof handle === "object" && handle !== null && "detail" in handle;
}
