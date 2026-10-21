import {
  BookOpenText,
  ClipboardList,
  Download,
  FileText,
  FileUp,
  Globe,
  GraduationCap,
  Image,
  KeyRound,
  LayoutDashboard,
  Library,
  School,
  NotebookPen,
  ScrollText,
  Settings,
  ShieldCheck,
  Sparkles,
  SquarePen,
  Timer,
  User,
  UserCog,
  Users,
  WandSparkles,
  type LucideIcon,
} from "lucide-react";

/** ApiGroup is how the API reference draws one tag of the contract: its icon and the paths it covers. */
export type ApiGroup = Readonly<{ icon: LucideIcon; path: string }>;

/**
 * API_GROUPS has an entry for each tag `api/openapi.yaml` lists; the name and
 * the blurb of a tag are `apiReference.groups.<tag>.name` and `.blurb` in both
 * locales. `api-groups.test.ts` fails when a tag has no entry here or no
 * words there.
 */
export const API_GROUPS: Readonly<Record<string, ApiGroup>> = {
  public: { icon: Globe, path: "/public/*" },
  auth: { icon: KeyRound, path: "/auth/*" },
  "teacher-tests": { icon: FileText, path: "/teacher/tests/*" },
  "teacher-questions": { icon: Library, path: "/teacher/questions/*" },
  "teacher-imports": { icon: FileUp, path: "/teacher/imports/*" },
  "teacher-media": { icon: Image, path: "/teacher/media/*" },
  "teacher-assignments": { icon: ClipboardList, path: "/teacher/assignments/*" },
  "teacher-attempts": { icon: SquarePen, path: "/teacher/attempts/*" },
  "teacher-students": { icon: Users, path: "/teacher/students/*" },
  "teacher-classes": { icon: GraduationCap, path: "/teacher/classes/*" },
  "teacher-ai": { icon: Sparkles, path: "/teacher/ai/*" },
  "admin-users": { icon: UserCog, path: "/admin/users/*" },
  "admin-roles": { icon: ShieldCheck, path: "/admin/roles/*" },
  "admin-classes": { icon: School, path: "/admin/classes/*" },
  "admin-ai": { icon: WandSparkles, path: "/admin/ai/*" },
  "admin-docs": { icon: BookOpenText, path: "/admin/api-reference/*" },
  "admin-audit": { icon: ScrollText, path: "/admin/audit-log" },
  "admin-exports": { icon: Download, path: "/admin/data-exports/*" },
  "admin-overview": { icon: LayoutDashboard, path: "/admin/overview" },
  "admin-settings": { icon: Settings, path: "/admin/settings/*" },
  student: { icon: NotebookPen, path: "/app/classes/*" },
  "take-test": { icon: Timer, path: "/app/attempts/*" },
  me: { icon: User, path: "/me/*" },
};

/** FALLBACK_GROUP draws a tag a newer server reports and this client has no entry for. */
export const FALLBACK_GROUP: ApiGroup = { icon: BookOpenText, path: "" };
