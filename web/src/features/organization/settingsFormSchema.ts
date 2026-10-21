import { z } from "zod";
import type {
  OrganizationSettings,
  OrganizationSettingsInput,
  SignInPolicy,
} from "@/features/organization/api";
import { limitsSchema, limitsValues } from "@/features/organization/limits";

const DOMAIN =
  /^(?=.{1,253}$)[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$/;

const optional = (limit: number, message: string) =>
  z
    .string()
    .transform((value) => value.trim())
    .pipe(z.string().max(limit, message))
    .transform((value) => (value === "" ? null : value));

const logo = z
  .object({
    sha256: z.string(),
    type: z.literal("image/png"),
    preview: z.string().optional(),
  })
  .transform(({ sha256, type }) => ({ sha256, type }))
  .nullable();

const keptMonths = z.union([z.literal(12), z.literal(24)]).nullable();

/**
 * settingsFormSchema is the System settings form: the centre's profile and
 * logos, the sign-in policy, the retention and the usage limits the document
 * carries. The four sections edit one document, so they share one schema. A logo's
 * `preview` is the address of the PNG just uploaded, for the tile, and is
 * not sent. The rules only the server can judge, a website, an email address
 * and a phone number, are left to it and shown on their field.
 */
export const settingsFormSchema = z.object({
  organization: z.object({
    name: z
      .string()
      .trim()
      .min(1, "systemSettings.errors.nameRequired")
      .max(120, "systemSettings.errors.nameTooLong"),
    shortName: optional(40, "systemSettings.errors.shortNameTooLong"),
    website: optional(200, "systemSettings.errors.websiteTooLong"),
    contactEmail: optional(254, "systemSettings.errors.contactEmailTooLong"),
    frontDeskPhone: optional(30, "systemSettings.errors.frontDeskPhoneTooLong"),
    frontDeskHours: optional(120, "systemSettings.errors.frontDeskHoursTooLong"),
    address: optional(300, "systemSettings.errors.addressTooLong"),
    logo: z.object({ light: logo, dark: logo }),
  }),
  signIn: z
    .object({
      google: z.boolean(),
      password: z.boolean(),
      forcePasswordChange: z.boolean(),
      joinCodes: z.boolean(),
      allowedGoogleDomains: z
        .array(z.string().regex(DOMAIN, "systemSettings.errors.domainInvalid"))
        .max(50, "systemSettings.errors.domainsTooMany"),
    })
    .refine((policy) => policy.google || policy.password, {
      message: "systemSettings.errors.noMethod",
      path: ["google"],
    }),
  retention: z.object({
    keepAnswersMonths: keptMonths,
    keepFocusEventsMonths: z.union([
      z.literal(6),
      z.literal(12),
      z.literal(13),
      z.literal(24),
    ]),
    anonymiseDisabledAfterMonths: keptMonths,
  }),
  limits: limitsSchema,
});

/** SettingsFormValues is what the System settings form's fields hold. */
export type SettingsFormValues = z.input<typeof settingsFormSchema>;

/** SettingsFormOutput is the whole document the form submits, but for its revision. */
export type SettingsFormOutput = z.output<typeof settingsFormSchema>;

type Expect<T extends true> = T;
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;

/** SettingsFormMatchesTheContract pins the form's output to every field of `PUT /admin/settings` but the revision. */
export type SettingsFormMatchesTheContract = Expect<
  Equal<
    SettingsFormOutput,
    Required<Omit<OrganizationSettingsInput, "expectedRevision">>
  >
>;

/** formValues is the form's fields for a stored document, with an optional text shown empty. */
export function formValues(saved: OrganizationSettings): SettingsFormValues {
  const { organization, signIn, retention, limits, limitDefaults } = saved;
  return {
    organization: {
      name: organization.name,
      shortName: organization.shortName ?? "",
      website: organization.website ?? "",
      contactEmail: organization.contactEmail ?? "",
      frontDeskPhone: organization.frontDeskPhone ?? "",
      frontDeskHours: organization.frontDeskHours ?? "",
      address: organization.address ?? "",
      logo: { ...organization.logo },
    },
    signIn: { ...signIn, allowedGoogleDomains: [...signIn.allowedGoogleDomains] },
    retention: { ...retention },
    limits: limitsValues(limits, limitDefaults),
  };
}

/** settingsInput is the body of `PUT /admin/settings` for a validated form, read at `revision`. */
export function settingsInput(
  values: SettingsFormOutput,
  revision: number,
): OrganizationSettingsInput {
  return { ...values, expectedRevision: revision };
}

/** domainOf is a Google domain as it is kept: lower-case, without a leading `@`. */
export function domainOf(entry: string): string {
  return entry.trim().toLowerCase().replace(/^@+/, "");
}

/** domainsValid reports whether every domain of `policy` is one the server takes. */
export function domainsValid(policy: SignInPolicy): boolean {
  return (
    policy.allowedGoogleDomains.length <= 50 &&
    policy.allowedGoogleDomains.every((domain) => DOMAIN.test(domain))
  );
}

/** samePolicy reports whether two sign-in policies are equal, the domains in order. */
export function samePolicy(a: SignInPolicy, b: SignInPolicy): boolean {
  return (
    a.google === b.google &&
    a.password === b.password &&
    a.forcePasswordChange === b.forcePasswordChange &&
    a.joinCodes === b.joinCodes &&
    a.allowedGoogleDomains.length === b.allowedGoogleDomains.length &&
    a.allowedGoogleDomains.every((domain, at) => domain === b.allowedGoogleDomains[at])
  );
}
