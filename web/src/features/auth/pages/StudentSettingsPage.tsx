import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams } from "react-router";
import { Segmented } from "@/components/ui/segmented";
import {
  StudentAppearanceSection,
  StudentProfileSection,
  StudentSignInSection,
} from "@/features/auth/components/StudentSettingsSections";
import { useMediaQuery } from "@/hooks/useMediaQuery";

const BASE = "/app/settings";
const SECTIONS = [
  { id: "profile", label: "student.settings.sections.profile" },
  { id: "sign-in", label: "student.settings.sections.signIn" },
  { id: "appearance", label: "student.settings.sections.appearance" },
] as const;

type SectionId = (typeof SECTIONS)[number]["id"];

const MOVED: ReadonlyMap<string, SectionId> = new Map([
  ["security", "sign-in"],
  ["preferences", "profile"],
]);

function isSection(slug: string): slug is SectionId {
  return SECTIONS.some((section) => section.id === slug);
}

function activeSection(slug: string | undefined): SectionId {
  if (slug === undefined) return "profile";
  if (isSection(slug)) return slug;
  return MOVED.get(slug) ?? "profile";
}

function pathOf(id: SectionId): string {
  return id === "profile" ? BASE : `${BASE}/${id}`;
}

/**
 * StudentSettingsPage is the student's Settings, as the design deck draws
 * it: a 760px column with a switcher over the Profile, Sign-in and
 * Appearance sections, each at `/app/settings/:section`, Profile also at the
 * bare path. Every section stays mounted, so what is typed in one survives a
 * visit to another and a resize across 768px. The slugs of the screen this
 * replaces redirect: `security` to `sign-in`, and `preferences` to Profile,
 * where the language went. Any other slug redirects to Profile, which
 * includes `notifications` until that module ships. Below 768px the shell's
 * header carries the title, and the page's own heading is for screen readers.
 */
export default function StudentSettingsPage() {
  const { t } = useTranslation();
  const { section } = useParams();
  const navigate = useNavigate();
  const wide = useMediaQuery("(min-width: 768px)");
  const known = section === undefined || isSection(section);
  const active = activeSection(section);

  return (
    <div className="mx-auto flex w-full max-w-190 flex-col gap-4 [&_button]:min-h-auto [&_button]:min-w-auto">
      {!known && <Navigate to={pathOf(active)} replace />}
      <h1
        className={
          wide ? "text-h1-student font-semibold tracking-[-0.02em]" : "sr-only"
        }
      >
        {t("nav.settings")}
      </h1>
      <Segmented
        label={t("settings.navigation")}
        size="lg"
        value={active}
        options={SECTIONS.map(({ id, label }) => ({ value: id, label: t(label) }))}
        onChange={(id) => {
          if (id !== active) void navigate(pathOf(id as SectionId));
        }}
        className="max-w-full self-start overflow-x-auto [&>button]:outline-offset-1!"
      />
      <div hidden={active !== "profile"}>
        <StudentProfileSection />
      </div>
      <div hidden={active !== "sign-in"}>
        <StudentSignInSection />
      </div>
      <div hidden={active !== "appearance"}>
        <StudentAppearanceSection />
      </div>
    </div>
  );
}
