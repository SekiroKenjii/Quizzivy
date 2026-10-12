import { afterEach, beforeEach, expect, it } from "vitest";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { StudentAppearanceSection } from "@/features/auth/components/StudentSettingsSections";
import { Toaster, toast } from "@/components/ui/sonner";
import { useAuthStore } from "@/stores/auth";
import i18n from "@/lib/i18n";
import { studentUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

function gate() {
  let release!: () => void;
  const promise = new Promise<void>((resolve) => {
    release = resolve;
  });
  return { promise, release };
}

function currentToast() {
  const element = document.querySelector<HTMLElement>(
    '[data-sonner-toast]:not([data-removed="true"])',
  );
  expect(element).not.toBeNull();
  return element!;
}

beforeEach(async () => {
  await i18n.changeLanguage("vi");
  useAuthStore.getState().setSession("toast-owner", studentUser);
});

afterEach(async () => {
  cleanup();
  useAuthStore.getState().clearSession();
  toast.dismiss();
  localStorage.clear();
});

for (const retry of [false, true]) {
  it(
    retry
      ? "keeps a public toast Retry saving until acknowledgment, then removes Retry and expires"
      : "keeps an unacknowledged preference toast and naturally expires its saved acknowledgment",
    async () => {
      const reply = gate();
      const finished = gate();
      const bodies: unknown[] = [];
      let entered = false;
      server.use(
        http.patch("http://localhost:8080/me/preferences", async ({ request }) => {
          bodies.push(await request.json());
          if (retry && bodies.length === 1) return HttpResponse.error();
          entered = true;
          try {
            await reply.promise;
            return contractJson("/me/preferences", "patch", 200, { theme: "dark" });
          } finally {
            finished.release();
          }
        }),
      );
      render(
        <>
          <StudentAppearanceSection />
          <Toaster />
        </>,
      );
      const user = userEvent.setup();
      const dark = screen.getByRole("button", { name: "Tối" });
      try {
        await user.click(dark);
        if (retry) {
          await waitFor(() =>
            expect(
              within(currentToast()).getByRole("button", {
                name: i18n.t("common.retry"),
              }),
            ).toBeInTheDocument(),
          );
          await user.click(
            within(currentToast()).getByRole("button", {
              name: i18n.t("common.retry"),
            }),
          );
          await user.hover(dark);
        }
        await waitFor(() => expect(entered).toBe(true));
        await waitFor(() =>
          expect(
            within(currentToast()).getByText(i18n.t("settings.preferenceSaving")),
          ).toBeInTheDocument(),
        );
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 5000));
        });
        expect(
          within(currentToast()).getByText(i18n.t("settings.preferenceSaving")),
        ).toBeInTheDocument();
        expect(
          within(currentToast()).queryByRole("button", {
            name: i18n.t("common.retry"),
          }),
        ).toBeNull();
        reply.release();
        await waitFor(() =>
          expect(
            within(currentToast()).getByText(i18n.t("settings.preferenceSaved")),
          ).toBeInTheDocument(),
        );
        expect(
          within(currentToast()).queryByRole("button", {
            name: i18n.t("common.retry"),
          }),
        ).toBeNull();
        expect(useAuthStore.getState().user?.preferences?.theme).toBe("dark");
        expect(bodies).toEqual(
          retry ? [{ theme: "dark" }, { theme: "dark" }] : [{ theme: "dark" }],
        );
        await user.tab();
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 1000));
        });
        expect(
          within(currentToast()).getByText(i18n.t("settings.preferenceSaved")),
        ).toBeInTheDocument();
        await waitFor(
          () =>
            expect(
              document.querySelector('[data-sonner-toast]:not([data-removed="true"])'),
            ).toBeNull(),
          { timeout: 5000 },
        );
      } finally {
        reply.release();
        if (entered) await finished.promise;
      }
    },
    15000,
  );
}
