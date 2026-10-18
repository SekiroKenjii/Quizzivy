import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { teacherUser } from "@tests/support/fixtures";
import { BASE, renderStudents, rowOf, serveStudents, student } from "./support";
import "@/lib/i18n";

const HAN = student("018f0000-0000-7000-8000-0000000000e1", "Phạm Gia Hân", {
  email: "han.pham@gmail.com",
});

let patches: unknown[] = [];

beforeEach(() => {
  patches = [];
  serveStudents([HAN]);
  server.use(
    http.patch(`${BASE}/teacher/students/${HAN.id}`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push(body);
      return contractJson("/teacher/students/{id}", "patch", 200, { ...HAN, ...body });
    }),
  );
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

async function openRecord(user: ReturnType<typeof renderStudents>["user"]) {
  await user.click(within(await rowOf("Phạm Gia Hân")).getByText("Phạm Gia Hân"));
  const sheet = await screen.findByRole("dialog", { name: "Phạm Gia Hân" });
  await user.click(within(sheet).getByRole("button", { name: "Sửa thông tin" }));
  return sheet;
}

describe("G-07a: editing a student in their record", () => {
  it("turns the header into two fields and writes only on Lưu", async () => {
    const { user } = renderStudents(teacherUser);
    const sheet = await openRecord(user);

    expect(within(sheet).getByText(/Email cũng là tên đăng nhập/)).toBeInTheDocument();
    const email = within(sheet).getByLabelText("Email");
    await user.clear(email);
    await user.type(email, "han@example.com");
    expect(patches).toHaveLength(0);

    await user.click(within(sheet).getByRole("button", { name: "Lưu" }));
    await waitFor(() =>
      expect(patches).toEqual([{ fullName: "Phạm Gia Hân", email: "han@example.com" }]),
    );
  });

  it("refuses an email that is not one, before asking the server", async () => {
    const { user } = renderStudents(teacherUser);
    const sheet = await openRecord(user);

    const email = within(sheet).getByLabelText("Email");
    await user.clear(email);
    await user.type(email, "not-an-email");
    await user.click(within(sheet).getByRole("button", { name: "Lưu" }));

    expect(
      await within(sheet).findByText("Email chưa đúng định dạng."),
    ).toBeInTheDocument();
    expect(patches).toHaveLength(0);
  });

  describe("a student made before the address rule, whose stored address holds an invisible character", () => {
    const STORED = "mai.le️@gmail.com";
    const MAI = student("018f0000-0000-7000-8000-0000000000e2", "Lê Mai", {
      email: STORED,
    });

    beforeEach(() => {
      server.use(
        http.get(`${BASE}/teacher/students`, () =>
          HttpResponse.json({
            items: [MAI],
            page: 1,
            pageSize: 20,
            total: 1,
            facets: { total: 1, activeLast7Days: 0 },
          }),
        ),
        http.get(`${BASE}/teacher/students/${MAI.id}`, () => HttpResponse.json(MAI)),
        http.patch(`${BASE}/teacher/students/${MAI.id}`, async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          patches.push(body);
          return HttpResponse.json({ ...MAI, ...body });
        }),
      );
    });

    async function openMai(user: ReturnType<typeof renderStudents>["user"]) {
      await user.click(within(await rowOf("Lê Mai")).getByText("Lê Mai"));
      const sheet = await screen.findByRole("dialog", { name: "Lê Mai" });
      await user.click(within(sheet).getByRole("button", { name: "Sửa thông tin" }));
      return sheet;
    }

    it("saves a new name with the stored address sent back as it is", async () => {
      const { user } = renderStudents(teacherUser);
      const sheet = await openMai(user);

      const name = within(sheet).getByLabelText("Họ và tên");
      await user.clear(name);
      await user.type(name, "Lê Thị Mai");
      await user.click(within(sheet).getByRole("button", { name: "Lưu" }));

      await waitFor(() =>
        expect(patches).toEqual([{ fullName: "Lê Thị Mai", email: STORED }]),
      );
    });

    it("still refuses another address that holds one, before asking the server", async () => {
      const { user } = renderStudents(teacherUser);
      const sheet = await openMai(user);

      const email = within(sheet).getByLabelText("Email");
      await user.clear(email);
      await user.type(email, "mai.khac️@gmail.com");
      await user.click(within(sheet).getByRole("button", { name: "Lưu" }));

      expect(
        await within(sheet).findByText("Email chưa đúng định dạng."),
      ).toBeInTheDocument();
      expect(patches).toHaveLength(0);
    });
  });
});
