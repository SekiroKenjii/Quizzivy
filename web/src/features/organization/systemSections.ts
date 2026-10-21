import { Building2, Gauge, LogIn, ShieldCheck, type LucideIcon } from "lucide-react";
import { ADMIN_PATHS } from "@/app/adminPaths";

/**
 * SystemSection is one section of System settings: its id, which is also the
 * last segment of its address, the label and icon of its link, and the
 * first segment of the form paths it edits.
 */
export type SystemSection = Readonly<{
  id: string;
  label: string;
  icon: LucideIcon;
  to: string;
  owns: string;
}>;

/**
 * SYSTEM_SECTIONS are the sections System settings shows, in the order of its
 * navigation. A section that has not shipped is not listed (DG-157), and its
 * address is replaced by the bare one.
 */
export const SYSTEM_SECTIONS: readonly SystemSection[] = [
  {
    id: "organization",
    label: "systemSettings.sections.organization",
    icon: Building2,
    to: `${ADMIN_PATHS.settings}/organization`,
    owns: "organization",
  },
  {
    id: "sign-in",
    label: "systemSettings.sections.signIn",
    icon: LogIn,
    to: `${ADMIN_PATHS.settings}/sign-in`,
    owns: "signIn",
  },
  {
    id: "data",
    label: "systemSettings.sections.data",
    icon: ShieldCheck,
    to: `${ADMIN_PATHS.settings}/data`,
    owns: "retention",
  },
  {
    id: "plan",
    label: "systemSettings.sections.plan",
    icon: Gauge,
    to: `${ADMIN_PATHS.settings}/plan`,
    owns: "limits",
  },
];

/** sectionOfField is the section that edits the form path `path`, or undefined. */
export function sectionOfField(path: string): SystemSection | undefined {
  return SYSTEM_SECTIONS.find(
    ({ owns }) => path === owns || path.startsWith(`${owns}.`),
  );
}
