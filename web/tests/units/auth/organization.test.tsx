import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SystemFrame } from "@/app/pages/SystemFrame";
import { AuthLayout } from "@/features/auth/AuthLayout";
import ForgotPasswordPage from "@/features/auth/pages/ForgotPasswordPage";
import LoginPage from "@/features/auth/pages/LoginPage";
import JoinPage from "@/features/join/pages/JoinPage";
import { writeThemePreference } from "@/lib/theme";
import {
  logoFor,
  PUBLIC_ORGANIZATION_KEY,
  publicOrganizationQuery,
  resolveOrganization,
  type PublicOrganization,
} from "@/lib/organization";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { studentUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const build = vi.hoisted(() => ({
  orgName: null as string | null,
  frontDesk: { phone: null as string | null, hours: null as string | null },
}));
vi.mock("@/lib/config", () => ({ config: build }));

const google = vi.hoisted(() => ({ available: true }));
vi.mock("@/features/auth/google/useGoogleSignIn", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/features/auth/google/useGoogleSignIn")>()),
  googleSignInAvailable: () => google.available,
}));

const BASE = "http://localhost:8080";
const LIGHT = "/public/organization/logo?variant=light&v=5e88a1";
const DARK = "/public/organization/logo?variant=dark&v=7c10b2";

function organization(overrides: Partial<PublicOrganization> = {}): PublicOrganization {
  return {
    name: "Quizzivy",
    shortName: null,
    website: null,
    contactEmail: null,
    frontDeskPhone: null,
    frontDeskHours: null,
    address: null,
    logo: { lightUrl: null, darkUrl: null },
    signIn: { google: true, password: true },
    joinCodes: true,
    ...overrides,
  };
}

let requests = 0;

function answers(overrides: Partial<PublicOrganization> = {}) {
  server.use(
    http.get(`${BASE}/public/organization`, () => {
      requests += 1;
      return contractJson("/public/organization", "get", 200, organization(overrides));
    }),
  );
}

function down() {
  server.use(
    http.get(`${BASE}/public/organization`, () => {
      requests += 1;
      return HttpResponse.error();
    }),
  );
}

function pending() {
  server.use(
    http.get(`${BASE}/public/organization`, () => {
      requests += 1;
      return new Promise<Response>(() => undefined);
    }),
  );
}

function newClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderPage(
  element: React.ReactElement,
  { path = "/", client = newClient() }: { path?: string; client?: QueryClient } = {},
) {
  const router = createMemoryRouter(
    [
      { path: "/join/:code?", element },
      { path: "/login", element },
      { path: "/forgot-password", element },
      { path: "*", element },
      { path: "/app", element: <p>student home</p> },
    ],
    { initialEntries: [path] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...view, client };
}

const logoImages = (container: HTMLElement) =>
  [...container.querySelectorAll<HTMLImageElement>("img")].filter((img) =>
    img.src.includes("/public/organization/logo"),
  );

beforeEach(() => {
  requests = 0;
  build.orgName = null;
  build.frontDesk = { phone: null, hours: null };
  google.available = true;
  answers();
});

afterEach(() => {
  writeThemePreference("light");
  useAuthStore.setState({ user: null });
});

describe("resolveOrganization", () => {
  it("prefers the centre's own line, then the build value, then nothing", () => {
    build.orgName = "Build Centre";
    build.frontDesk = { phone: "028 0000 0000", hours: "T2–T6" };

    const own = resolveOrganization(
      organization({
        name: "Sao Mai English",
        frontDeskPhone: "028 3812 4567",
        frontDeskHours: "T2–T7, 8:00–20:00",
      }),
    );
    expect(own.name).toBe("Sao Mai English");
    expect(own.frontDesk).toEqual({
      phone: "028 3812 4567",
      hours: "T2–T7, 8:00–20:00",
    });

    const unset = resolveOrganization(organization({ frontDeskPhone: "   " }));
    expect(unset.frontDesk).toEqual({ phone: "028 0000 0000", hours: "T2–T6" });

    build.orgName = null;
    build.frontDesk = { phone: null, hours: null };
    const neither = resolveOrganization(organization());
    expect(neither.name).toBeNull();
    expect(neither.frontDesk).toEqual({ phone: null, hours: null });
  });

  it("counts the name the settings row is born with as unset", () => {
    build.orgName = "Quizzivy English Centre";
    expect(resolveOrganization(organization({ name: "Quizzivy" })).name).toBe(
      "Quizzivy English Centre",
    );
    expect(resolveOrganization(organization({ name: "  QUIZZIVY " })).name).toBe(
      "Quizzivy English Centre",
    );
    build.orgName = null;
    expect(resolveOrganization(organization({ name: "Quizzivy" })).name).toBeNull();
  });

  it("turns every line into its build value and every way in on without an answer", () => {
    build.orgName = "Build Centre";
    build.frontDesk = { phone: "028 0000 0000", hours: null };
    expect(resolveOrganization(undefined)).toEqual({
      name: "Build Centre",
      logo: { light: null, dark: null },
      frontDesk: { phone: "028 0000 0000", hours: null },
      signIn: { google: true, password: true },
      joinCodes: true,
    });
  });

  it("puts a logo path on the API's origin", () => {
    const resolved = resolveOrganization(
      organization({ logo: { lightUrl: LIGHT, darkUrl: DARK } }),
    );
    expect(resolved.logo).toEqual({ light: `${BASE}${LIGHT}`, dark: `${BASE}${DARK}` });
  });

  it("draws the logo made for the theme, and flips the other when that is all there is", () => {
    const both = resolveOrganization(
      organization({ logo: { lightUrl: LIGHT, darkUrl: DARK } }),
    );
    expect(logoFor(both, "light")).toEqual({ url: `${BASE}${LIGHT}`, flipped: false });
    expect(logoFor(both, "dark")).toEqual({ url: `${BASE}${DARK}`, flipped: false });

    const lightOnly = resolveOrganization(
      organization({ logo: { lightUrl: LIGHT, darkUrl: null } }),
    );
    expect(logoFor(lightOnly, "light")).toEqual({
      url: `${BASE}${LIGHT}`,
      flipped: false,
    });
    expect(logoFor(lightOnly, "dark")).toEqual({
      url: `${BASE}${LIGHT}`,
      flipped: true,
    });

    const darkOnly = resolveOrganization(
      organization({ logo: { lightUrl: null, darkUrl: DARK } }),
    );
    expect(logoFor(darkOnly, "light")).toEqual({
      url: `${BASE}${DARK}`,
      flipped: true,
    });

    expect(logoFor(resolveOrganization(organization()), "dark")).toBeNull();
  });

  it("keeps the answer for five minutes", () => {
    expect(publicOrganizationQuery().staleTime).toBe(300_000);
  });
});

describe("the brand panel", () => {
  function renderPanel(client?: QueryClient) {
    return renderPage(
      <AuthLayout>
        <h1>form</h1>
      </AuthLayout>,
      client ? { client } : {},
    );
  }

  it("prints the centre's name and its light logo", async () => {
    answers({ name: "Sao Mai English", logo: { lightUrl: LIGHT, darkUrl: DARK } });
    const { container } = renderPanel();
    expect(await screen.findByText("Sao Mai English")).toBeInTheDocument();
    const [logo, ...others] = logoImages(container);
    expect(others).toHaveLength(0);
    expect(logo).toHaveAttribute("src", `${BASE}${LIGHT}`);
    expect(logo).toHaveAttribute("alt", "");
  });

  it("draws the dark logo in the dark theme", async () => {
    writeThemePreference("dark");
    answers({ name: "Sao Mai English", logo: { lightUrl: LIGHT, darkUrl: DARK } });
    const { container } = renderPanel();
    await screen.findByText("Sao Mai English");
    expect(logoImages(container)[0]).toHaveAttribute("src", `${BASE}${DARK}`);
  });

  it("puts a logo made for the other theme on a tile of the opposite ground", async () => {
    answers({ name: "Sao Mai English", logo: { lightUrl: LIGHT, darkUrl: null } });
    const light = renderPanel();
    await screen.findByText("Sao Mai English");
    expect(logoImages(light.container)[0]?.parentElement).toHaveClass("bg-card");
    light.unmount();

    writeThemePreference("dark");
    const dark = renderPanel();
    await screen.findByText("Sao Mai English");
    expect(logoImages(dark.container)[0]).toHaveAttribute("src", `${BASE}${LIGHT}`);
    expect(logoImages(dark.container)[0]?.parentElement).toHaveClass("bg-fg");
  });

  it("leaves a logo out when it will not load, and keeps the name", async () => {
    answers({ name: "Sao Mai English", logo: { lightUrl: LIGHT, darkUrl: null } });
    const { container } = renderPanel();
    await screen.findByText("Sao Mai English");
    fireEvent.error(logoImages(container)[0]!);
    expect(logoImages(container)).toHaveLength(0);
    expect(screen.getByText("Sao Mai English")).toBeInTheDocument();
  });

  it("names the product on a logo that stands alone", async () => {
    answers({ name: "Quizzivy", logo: { lightUrl: LIGHT, darkUrl: null } });
    const { container } = renderPanel();
    await waitFor(() => expect(logoImages(container)).toHaveLength(1));
    expect(logoImages(container)[0]).toHaveAttribute("alt", "Quizzivy");
  });

  it("draws no line when the centre has neither a name nor a logo", async () => {
    const { container } = renderPanel();
    await waitFor(() => expect(requests).toBe(1));
    expect(logoImages(container)).toHaveLength(0);
    expect(container.querySelector("aside")?.textContent).not.toContain("Quizzivy ");
    expect(
      container.querySelectorAll("aside p"),
      "only the headline remains",
    ).toHaveLength(1);
  });

  it("prints the build name while the answer is out", async () => {
    build.orgName = "Quizzivy English Centre";
    pending();
    renderPanel();
    expect(screen.getByText("Quizzivy English Centre")).toBeInTheDocument();
    await waitFor(() => expect(requests).toBe(1));
    expect(screen.getByText("Quizzivy English Centre")).toBeInTheDocument();
  });

  it("prints the build name when the request fails", async () => {
    build.orgName = "Quizzivy English Centre";
    down();
    renderPanel();
    await waitFor(() => expect(requests).toBe(1));
    await act(() => Promise.resolve());
    expect(screen.getByText("Quizzivy English Centre")).toBeInTheDocument();
  });

  it("keeps a cached answer through a refresh that fails", async () => {
    const client = newClient();
    client.setQueryData(
      PUBLIC_ORGANIZATION_KEY,
      organization({
        name: "Sao Mai English",
        logo: { lightUrl: LIGHT, darkUrl: null },
      }),
    );
    down();
    const { container } = renderPanel(client);
    expect(screen.getByText("Sao Mai English")).toBeInTheDocument();
    await act(() => client.refetchQueries({ queryKey: PUBLIC_ORGANIZATION_KEY }));
    expect(requests).toBe(1);
    expect(screen.getByText("Sao Mai English")).toBeInTheDocument();
    expect(logoImages(container)).toHaveLength(1);
  });

  it("asks once for the pages that share it", async () => {
    answers({ name: "Sao Mai English" });
    const client = newClient();
    const first = renderPanel(client);
    await screen.findByText("Sao Mai English");
    first.unmount();
    renderPanel(client);
    expect(screen.getByText("Sao Mai English")).toBeInTheDocument();
    await act(() => Promise.resolve());
    expect(requests).toBe(1);
  });
});

describe("the system pages' caption", () => {
  function renderSystem(client?: QueryClient) {
    return renderPage(
      <SystemFrame art={<span>art</span>}>
        <h1>gone</h1>
      </SystemFrame>,
      client ? { client } : {},
    );
  }

  it("prints the centre's logo and name", async () => {
    answers({ name: "Sao Mai English", logo: { lightUrl: LIGHT, darkUrl: null } });
    const { container } = renderSystem();
    expect(await screen.findByText("Sao Mai English")).toBeInTheDocument();
    expect(logoImages(container)).toHaveLength(1);
  });

  it("falls back to the build name with the API down", async () => {
    build.orgName = "Quizzivy English Centre";
    down();
    renderSystem();
    await waitFor(() => expect(requests).toBe(1));
    expect(screen.getByText("Quizzivy English Centre")).toBeInTheDocument();
  });

  it("draws nothing in the caption when nothing is set", async () => {
    const { container } = renderSystem();
    await waitFor(() => expect(requests).toBe(1));
    expect(container.querySelector("aside > div:last-child")).toBeEmptyDOMElement();
  });
});

describe("the front desk on Forgot password", () => {
  const phone = "028 3812 4567";
  const hours = "T2–T7, 8:00–20:00";

  it("shows the centre's phone and hours", async () => {
    answers({ frontDeskPhone: phone, frontDeskHours: hours });
    renderPage(<ForgotPasswordPage />);
    expect(
      await screen.findByText(`${phone} · ${hours}`, { exact: false }),
    ).toBeVisible();
    expect(screen.getByText("Quầy lễ tân")).toBeInTheDocument();
  });

  it("takes the line the centre left empty from the build values", async () => {
    build.frontDesk = { phone: "028 0000 0000", hours: "build hours" };
    answers({ frontDeskPhone: phone });
    renderPage(<ForgotPasswordPage />);
    expect(
      await screen.findByText(`${phone} · build hours`, { exact: false }),
    ).toBeVisible();
  });

  it("shows the build values before the answer and with the API down", async () => {
    build.frontDesk = { phone: "028 0000 0000", hours: null };
    down();
    renderPage(<ForgotPasswordPage />);
    expect(screen.getByText("028 0000 0000", { exact: false })).toBeInTheDocument();
    await waitFor(() => expect(requests).toBe(1));
    expect(screen.getByText("028 0000 0000", { exact: false })).toBeInTheDocument();
  });

  it("hides the front desk when neither has a line", async () => {
    renderPage(<ForgotPasswordPage />);
    await waitFor(() => expect(requests).toBe(1));
    expect(screen.queryByText("Quầy lễ tân")).toBeNull();
  });
});

describe("the sign-in page", () => {
  const google_ = () => screen.queryByRole("button", { name: "Tiếp tục với Google" });
  const password = () => screen.queryByLabelText("Mật khẩu");
  const divider = () => screen.queryByText("hoặc dùng email");
  const joinLink = () => screen.queryByRole("link", { name: "Tham gia lớp" });
  const noMethod = () => screen.queryByText(/chưa có cách đăng nhập nào dùng được/);

  it("offers both while the answer is out", () => {
    pending();
    renderPage(<LoginPage />, { path: "/login" });
    expect(google_()).toBeInTheDocument();
    expect(password()).toBeInTheDocument();
    expect(divider()).toBeInTheDocument();
    expect(noMethod()).toBeNull();
  });

  it("offers both with the API down", async () => {
    down();
    renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(requests).toBe(1));
    await act(() => Promise.resolve());
    expect(google_()).toBeInTheDocument();
    expect(password()).toBeInTheDocument();
  });

  it("offers both when both are on", async () => {
    answers({ signIn: { google: true, password: true } });
    renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(requests).toBe(1));
    expect(google_()).toBeInTheDocument();
    expect(password()).toBeInTheDocument();
    expect(divider()).toBeInTheDocument();
  });

  it("hides the Google button and the divider when Google is off", async () => {
    answers({ signIn: { google: false, password: true } });
    renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(google_()).toBeNull());
    expect(divider()).toBeNull();
    expect(password()).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đăng nhập" })).toBeEnabled();
    expect(noMethod()).toBeNull();
  });

  it("hides the password form and the divider when password sign-in is off", async () => {
    answers({ signIn: { google: true, password: false } });
    renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(password()).toBeNull());
    expect(screen.queryByLabelText("Email")).toBeNull();
    expect(screen.queryByRole("link", { name: "Quên mật khẩu?" })).toBeNull();
    expect(divider()).toBeNull();
    expect(google_()).toBeInTheDocument();
    expect(noMethod()).toBeNull();
  });

  it("says no way in is available when the only one on cannot be offered", async () => {
    google.available = false;
    answers({ signIn: { google: true, password: false } });
    renderPage(<LoginPage />, { path: "/login" });
    expect(
      await screen.findByText(/chưa có cách đăng nhập nào dùng được/),
    ).toBeVisible();
    expect(google_()).toBeNull();
    expect(password()).toBeNull();
  });

  it("keeps the password form when Google is on but not configured here", async () => {
    google.available = false;
    answers({ signIn: { google: true, password: true } });
    renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(requests).toBe(1));
    expect(google_()).toBeNull();
    expect(divider()).toBeNull();
    expect(password()).toBeInTheDocument();
  });

  it("links to joining a class only while joining with a code is on", async () => {
    answers({ joinCodes: false });
    const off = renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(joinLink()).toBeNull());
    expect(screen.queryByText("Bạn có mã lớp?")).toBeNull();
    off.unmount();

    answers({ joinCodes: true });
    renderPage(<LoginPage />, { path: "/login" });
    await waitFor(() => expect(requests).toBe(2));
    expect(joinLink()).toHaveAttribute("href", "/join");
  });
});

describe("the join page", () => {
  const heading = () =>
    screen.queryByRole("heading", { name: "Tham gia bằng mã đang tắt" });

  it("shows the code form while joining with a code is on", async () => {
    answers({ joinCodes: true });
    renderPage(<JoinPage />, { path: "/join" });
    await waitFor(() => expect(requests).toBe(1));
    expect(
      screen.getByRole("heading", { name: "Tham gia lớp học" }),
    ).toBeInTheDocument();
    expect(heading()).toBeNull();
  });

  it("shows the code form with the API down", async () => {
    down();
    renderPage(<JoinPage />, { path: "/join" });
    await waitFor(() => expect(requests).toBe(1));
    await act(() => Promise.resolve());
    expect(
      screen.getByRole("heading", { name: "Tham gia lớp học" }),
    ).toBeInTheDocument();
  });

  it("says joining with a code is off, with a way back to sign in", async () => {
    answers({ joinCodes: false });
    renderPage(<JoinPage />, { path: "/join/ABCD2345" });
    expect(
      await screen.findByRole("heading", { name: "Tham gia bằng mã đang tắt" }),
    ).toBeVisible();
    expect(screen.queryByLabelText("Mã lớp")).toBeNull();
    expect(screen.getByRole("link", { name: "Quay lại đăng nhập" })).toHaveAttribute(
      "href",
      "/login",
    );
  });

  it("sends a signed-in student back home from the off page", async () => {
    useAuthStore.setState({ user: studentUser });
    answers({ joinCodes: false });
    renderPage(<JoinPage />, { path: "/join" });
    await waitFor(() => expect(heading()).toBeInTheDocument());
    expect(screen.getByRole("link", { name: "Về trang chủ của tôi" })).toHaveAttribute(
      "href",
      "/app",
    );
  });
});
