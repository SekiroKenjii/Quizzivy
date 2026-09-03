import {
  GoogleSection,
  LanguageSection,
  PasswordSection,
} from "@/features/auth/components/SettingsSections";
import { SignOutButton } from "@/features/auth/SignOutButton";

/**
 * §9's settings: three sections, and no profile block -- a student's name and
 * email come from the teacher or from Google, and there is nothing here for
 * them to edit.
 */
export default function StudentSettingsPage() {
  return (
    <div className="space-y-3">
      <PasswordSection />
      <GoogleSection />
      <LanguageSection />
      <SignOutButton variant="outline" className="text-muted-foreground w-full" />
    </div>
  );
}
