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
 *
 * The deck's S-10 puts signing out at the bottom of this screen rather than in
 * the nav bar, where it sat one mis-tap away from the two things a student is
 * actually there to do.
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
