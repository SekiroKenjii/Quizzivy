import {
  GoogleSection,
  PasswordSection,
} from "@/features/auth/components/SettingsSections";
import { DevicesSection } from "@/features/settings/sections/Devices";

/**
 * SecuritySection is the teacher's Sign-in & security: the password, the
 * Google link and the signed-in devices, each its own card.
 */
export function SecuritySection() {
  return (
    <>
      <PasswordSection />
      <GoogleSection />
      <DevicesSection />
    </>
  );
}
