import { http } from "msw";
import { vi } from "vitest";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { assertContractRequest, contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

type Settings = components["schemas"]["OrganizationSettings"];
type Input = components["schemas"]["OrganizationSettingsInput"];
type Impact = components["schemas"]["SignInImpact"];

/** BASE is the API origin the tests' handlers answer on. */
export const BASE = "http://localhost:8080";

/** REQUEST_ID is a request id the error envelope accepts. */
export const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

/** SAVED is the settings document the page reads unless a test serves another. */
export const SAVED: Settings = {
  organization: {
    name: "Quizzivy English Centre",
    shortName: "Quizzivy",
    website: "quizzivy.com",
    contactEmail: "hello@quizzivy.com",
    frontDeskPhone: "028 7300 1234",
    frontDeskHours: "8:00 – 20:00 hằng ngày",
    address: "12 Nguyễn Huệ, Quận 1, TP.HCM",
    logo: { light: null, dark: null },
  },
  signIn: {
    google: true,
    password: true,
    forcePasswordChange: true,
    joinCodes: true,
    allowedGoogleDomains: ["quizzivy.com", "gmail.com"],
  },
  retention: {
    keepAnswersMonths: 24,
    keepFocusEventsMonths: 13,
    anonymiseDisabledAfterMonths: null,
  },
  revision: 7,
  updatedAt: "2026-10-14T09:00:00Z",
  updatedBy: adminUser.id,
};

/** NO_IMPACT is the count of a policy that costs no one a way in. */
export const NO_IMPACT: Impact = {
  affectedUsers: 0,
  affectedAdmins: 0,
  actorLocked: false,
  lockout: false,
};

/** HASH is the SHA-256 the logo upload answers for the light variant. */
export const HASH = "a".repeat(64);

/** DARK_HASH is the SHA-256 the logo upload answers for the dark variant. */
export const DARK_HASH = "b".repeat(64);

/** error is an error envelope the contract accepts. */
export function failure(
  code: string,
  message: string,
  details?: Record<string, unknown>,
) {
  return {
    error: { code, message, requestId: REQUEST_ID, ...(details ? { details } : {}) },
  };
}

/**
 * servesSettings answers `GET` and `PUT /admin/settings` from `document`:
 * a save replaces it, one revision higher, as the server does. It records
 * every `PUT` body, and each read's count. `put` replaces the save's answer.
 */
export function servesSettings(
  document: Settings = SAVED,
  put?: (body: Input) => Response | Promise<Response>,
) {
  const seen = { reads: 0, puts: [] as Input[], document };
  server.use(
    http.get(`${BASE}/admin/settings`, () => {
      seen.reads += 1;
      return contractJson("/admin/settings", "get", 200, seen.document);
    }),
    http.put(`${BASE}/admin/settings`, async ({ request }) => {
      const body = (await request.json()) as Input;
      assertContractRequest("/admin/settings", "put", body);
      seen.puts.push(body);
      if (put) return put(body);
      const { expectedRevision, ...rest } = body;
      seen.document = {
        ...rest,
        revision: expectedRevision + 1,
        updatedAt: "2026-10-15T09:00:00Z",
        updatedBy: adminUser.id,
      };
      return contractJson("/admin/settings", "put", 200, seen.document);
    }),
  );
  return seen;
}

/** servesImpact answers `POST /admin/settings/sign-in-impact` with `answer` and records the policies asked about. */
export function servesImpact(
  answer: Impact | ((policy: Input["signIn"]) => Impact) = NO_IMPACT,
) {
  const asked: Input["signIn"][] = [];
  server.use(
    http.post(`${BASE}/admin/settings/sign-in-impact`, async ({ request }) => {
      const body = (await request.json()) as { signIn: Input["signIn"] };
      assertContractRequest("/admin/settings/sign-in-impact", "post", body);
      asked.push(body.signIn);
      return contractJson(
        "/admin/settings/sign-in-impact",
        "post",
        200,
        typeof answer === "function" ? answer(body.signIn) : answer,
      );
    }),
  );
  return asked;
}

/**
 * servesLogoUpload answers `POST /admin/settings/logo` with a hash per variant
 * and records each upload's variant, file name, type and size, as the form
 * was built in the browser (the mocked network drops a file's name). `refuse`
 * answers instead, for the uploads it returns a response for. The caller
 * restores mocks when the test ends.
 */
export function servesLogoUpload(refuse?: (variant: string) => Response | undefined) {
  const sent: { variant: string; name: string; type: string; size: number }[] = [];
  const append = FormData.prototype.append as (
    this: FormData,
    ...args: unknown[]
  ) => void;
  let variant = "";
  vi.spyOn(FormData.prototype, "append").mockImplementation(function (
    this: FormData,
    ...args: unknown[]
  ) {
    const [name, value] = args;
    if (name === "variant") variant = String(value);
    if (name === "file" && value instanceof File)
      sent.push({ variant, name: value.name, type: value.type, size: value.size });
    append.apply(this, args);
  } as never);
  server.use(
    http.post(`${BASE}/admin/settings/logo`, async ({ request }) => {
      const answered = String((await request.formData()).get("variant"));
      const refused = refuse?.(answered);
      if (refused) return refused;
      return contractJson("/admin/settings/logo", "post", 200, {
        sha256: answered === "dark" ? DARK_HASH : HASH,
        type: "image/png",
        width: 512,
        height: 512,
      });
    }),
  );
  return sent;
}

/** servesPublicOrganization records how each read of `GET /public/organization` was made. */
export function servesPublicOrganization() {
  const reads: { cache: RequestCache }[] = [];
  server.use(
    http.get(`${BASE}/public/organization`, ({ request }) => {
      reads.push({ cache: request.cache });
      return contractJson("/public/organization", "get", 200, {
        name: SAVED.organization.name,
        shortName: null,
        website: null,
        contactEmail: null,
        frontDeskPhone: null,
        frontDeskHours: null,
        address: null,
        logo: { lightUrl: null, darkUrl: null },
        signIn: { google: true, password: true },
        joinCodes: true,
      });
    }),
  );
  return reads;
}

/** signInAsAdmin puts an Admin who may write the settings in the store. */
export function signInAsAdmin() {
  useAuthStore.getState().setSession("token-a", adminUser);
}
