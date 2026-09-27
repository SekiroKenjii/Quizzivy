import { useTranslation } from "react-i18next";
import { NavLink, useNavigate, useParams } from "react-router";
import { Shield, SlidersHorizontal, UserRound } from "lucide-react";
import { Avatar } from "@/components/ui/avatar";
import { SignOutButton } from "@/features/auth/SignOutButton";
import { formatDate } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";
import {
  ApiDocsSection,
  GoogleSection,
  LanguageSection,
  PasswordSection,
  ProfileSection,
} from "./SettingsSections";

const sections = [
  { id: "profile", icon: UserRound },
  { id: "security", icon: Shield },
  { id: "preferences", icon: SlidersHorizontal },
] as const;

/** SettingsPage keeps section forms mounted while URL navigation and viewport changes preserve edits. */
export function SettingsPage({
  base,
}: Readonly<{ base: "/admin/settings" | "/app/settings" }>) {
  const { t } = useTranslation();
  const { section } = useParams();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const active = sections.some((item) => item.id === section) ? section : "profile";
  const path = (id: string) => (id === "profile" ? base : `${base}/${id}`);
  return (
    <div className="w-full space-y-8">
      <header className="space-y-1.5 border-b pb-6">
        <h1 className="text-2xl font-semibold tracking-tight">{t("nav.settings")}</h1>
        <p className="text-muted-foreground text-sm">{t("settings.description")}</p>
      </header>
      <div className="grid min-w-0 gap-8 md:grid-cols-[12rem_minmax(0,1fr)] lg:gap-12">
        <div className="space-y-6">
          <nav
            aria-label={t("settings.navigation")}
            className="hidden space-y-1 md:block"
          >
            {sections.map(({ id, icon: Icon }) => (
              <NavLink
                key={id}
                to={path(id)}
                end
                className={cn(
                  "flex min-h-10 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors duration-150 motion-reduce:transition-none",
                  active === id
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                )}
              >
                <Icon className="size-4 shrink-0" aria-hidden="true" />
                {t(`settings.sections.${id}`)}
              </NavLink>
            ))}
          </nav>
          <div className="md:hidden">
            <label htmlFor="settings-section" className="sr-only">
              {t("settings.navigation")}
            </label>
            <select
              id="settings-section"
              value={active}
              onChange={(event) => void navigate(path(event.target.value))}
              className="bg-background min-h-11 w-full rounded-md border px-3 text-base"
            >
              {sections.map(({ id }) => (
                <option key={id} value={id}>
                  {t(`settings.sections.${id}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="w-full max-w-3xl min-w-0">
          <div hidden={active !== "profile"} className="settings-panel space-y-6">
            {user && (
              <div className="flex items-center gap-4 rounded-lg border p-5 shadow-sm">
                <Avatar name={user.fullName} size="lg" />
                <div className="min-w-0 space-y-1">
                  <p className="font-semibold break-words">{user.fullName}</p>
                  <p className="text-muted-foreground text-sm break-all">
                    {user.email}
                  </p>
                  <p className="text-muted-foreground text-xs">
                    {t(
                      user.role === "student"
                        ? "settings.roleStudent"
                        : "settings.roleTeacher",
                    )}{" "}
                    · {t("settings.since")} {formatDate(user.createdAt)}
                  </p>
                </div>
              </div>
            )}
            <ProfileSection />
          </div>
          <div hidden={active !== "security"} className="settings-panel space-y-8">
            <PasswordSection />
            <GoogleSection />
          </div>
          <div hidden={active !== "preferences"} className="settings-panel space-y-8">
            <LanguageSection />
            {base === "/admin/settings" ? <ApiDocsSection /> : null}
          </div>
          <div className="mt-8 border-t pt-5">
            <SignOutButton variant="outline" className="w-full sm:w-auto" />
          </div>
        </div>
      </div>
    </div>
  );
}
