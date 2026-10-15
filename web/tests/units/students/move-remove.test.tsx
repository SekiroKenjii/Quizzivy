import { afterEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import {
  BASE,
  CLASS_A,
  CLASS_B,
  inClass,
  menuOf,
  renderStudents,
  rowOf,
  serveStudents,
  student,
} from "./support";
import "@/lib/i18n";

const AN = student("018f0000-0000-7000-8000-0000000000e1", "Nguyễn Văn An", {
  classes: [inClass(CLASS_A)],
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function serveMembers(failAdd = false) {
  const calls: string[] = [];
  server.use(
    http.post(`${BASE}/teacher/classes/:id/members`, async ({ params, request }) => {
      const body = (await request.json()) as { userId: string };
      calls.push(`add ${String(params["id"])} ${body.userId}`);
      return failAdd
        ? HttpResponse.json(
            {
              error: {
                code: "INTERNAL",
                message: "Lỗi.",
                requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
              },
            },
            { status: 500 },
          )
        : new HttpResponse(null, { status: 204 });
    }),
    http.delete(`${BASE}/teacher/classes/:id/members/:userId`, ({ params }) => {
      calls.push(`remove ${String(params["id"])} ${String(params["userId"])}`);
      return new HttpResponse(null, { status: 204 });
    }),
  );
  return calls;
}

async function chooseClass(
  user: ReturnType<typeof renderStudents>["user"],
  name: string,
) {
  await user.click(screen.getByRole("combobox", { name: "Lớp" }));
  await user.click(await screen.findByRole("option", { name }));
}

describe("moving a student", () => {
  it("adds them to the chosen class, then takes them out of the old one", async () => {
    serveStudents([AN]);
    const calls = serveMembers();
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Chuyển lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Chuyển Nguyễn Văn An" });
    expect(
      within(dialog).getByRole("switch", { name: "Giữ học viên ở cả lớp hiện tại" }),
    ).not.toBeChecked();
    await chooseClass(user, CLASS_B.name);
    await user.click(within(dialog).getByRole("button", { name: "Chuyển" }));

    expect(
      await screen.findByText(`Đã chuyển Nguyễn Văn An sang ${CLASS_B.name}`),
    ).toBeInTheDocument();
    expect(calls).toEqual([
      `add ${CLASS_B.id} ${AN.id}`,
      `remove ${CLASS_A.id} ${AN.id}`,
    ]);
  });

  it("keeps the old class when asked to", async () => {
    serveStudents([AN]);
    const calls = serveMembers();
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Chuyển lớp" }));
    const dialog = await screen.findByRole("dialog");
    await chooseClass(user, CLASS_B.name);
    await user.click(
      within(dialog).getByRole("switch", { name: "Giữ học viên ở cả lớp hiện tại" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Chuyển" }));

    await waitFor(() => expect(calls).toEqual([`add ${CLASS_B.id} ${AN.id}`]));
  });

  it("names a student it could not move and stays open, taking them out of nothing", async () => {
    serveStudents([AN]);
    const calls = serveMembers(true);
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Chuyển lớp" }));
    const dialog = await screen.findByRole("dialog");
    await chooseClass(user, CLASS_B.name);
    await user.click(within(dialog).getByRole("button", { name: "Chuyển" }));

    const alert = await within(dialog).findByRole("alert");
    expect(alert).toHaveTextContent("Chưa chuyển được 1 học viên:");
    expect(alert).toHaveTextContent("Nguyễn Văn An: Lỗi.");
    expect(calls).toEqual([`add ${CLASS_B.id} ${AN.id}`]);
  });

  it("gives focus to the select-all box after a bulk Move clears the selection (QA-40-6)", async () => {
    const BINH = student("018f0000-0000-7000-8000-0000000000e2", "Trần Thị Bình", {
      classes: [inClass(CLASS_A)],
    });
    serveStudents([AN, BINH]);
    serveMembers();
    const { user } = renderStudents(teacherUser);

    for (const name of ["Nguyễn Văn An", "Trần Thị Bình"]) {
      await user.click(
        within(await rowOf(name)).getByRole("checkbox", { name: `Chọn ${name}` }),
      );
    }
    await user.click(screen.getByRole("button", { name: "Chuyển lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Chuyển 2 học viên" });
    await chooseClass(user, CLASS_B.name);
    await user.click(within(dialog).getByRole("button", { name: "Chuyển" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByRole("button", { name: "Chuyển lớp" })).toBeNull();
    expect(screen.getByRole("checkbox", { name: /Chọn tất cả/ })).toHaveFocus();
  });

  it("gives each student it could not move the server's own reason", async () => {
    const BINH = student("018f0000-0000-7000-8000-0000000000e2", "Trần Thị Bình", {
      classes: [inClass(CLASS_A)],
    });
    serveStudents([AN, BINH]);
    const refusal = (status: number, code: string, message: string) =>
      HttpResponse.json(
        {
          error: { code, message, requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e" },
        },
        { status },
      );
    server.use(
      http.post(`${BASE}/teacher/classes/:id/members`, async ({ request }) => {
        const body = (await request.json()) as { userId: string };
        return body.userId === AN.id
          ? refusal(403, "FORBIDDEN", "Học viên có quyền mà bạn không có.")
          : refusal(429, "RATE_LIMITED", "Bạn thao tác quá nhanh.");
      }),
    );
    const { user } = renderStudents(teacherUser);

    for (const name of ["Nguyễn Văn An", "Trần Thị Bình"]) {
      await user.click(
        within(await rowOf(name)).getByRole("checkbox", { name: `Chọn ${name}` }),
      );
    }
    await user.click(screen.getByRole("button", { name: "Chuyển lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Chuyển 2 học viên" });
    await chooseClass(user, CLASS_B.name);
    await user.click(within(dialog).getByRole("button", { name: "Chuyển" }));

    const alert = await within(dialog).findByRole("alert");
    expect(alert).toHaveTextContent("Chưa chuyển được 2 học viên:");
    expect(
      within(alert)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual([
      "Nguyễn Văn An: Học viên có quyền mà bạn không có.",
      "Trần Thị Bình: Bạn thao tác quá nhanh.",
    ]);
  });
});

describe("removing a student from their classes", () => {
  it("asks first, says the account and results stay, then removes every membership", async () => {
    serveStudents([AN]);
    const calls = serveMembers();
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Xoá khỏi lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Xoá Nguyễn Văn An khỏi các lớp của học viên?",
    });
    expect(dialog).toHaveTextContent("Tài khoản và kết quả vẫn được giữ.");
    expect(calls).toEqual([]);
    await user.click(within(dialog).getByRole("button", { name: "Xoá khỏi lớp" }));

    expect(
      await screen.findByText("Đã xoá Nguyễn Văn An khỏi các lớp"),
    ).toBeInTheDocument();
    expect(calls).toEqual([`remove ${CLASS_A.id} ${AN.id}`]);
  });

  it.each([
    [true, "Học viên có thể vào lại lớp bằng mã lớp."],
    [false, null],
  ])(
    "mentions the class code only while joining by code is %s (QC-U-30)",
    async (joinCodes, rejoin) => {
      server.use(
        http.get(`${BASE}/public/organization`, () =>
          contractJson("/public/organization", "get", 200, {
            name: "Quizzivy",
            shortName: null,
            website: null,
            contactEmail: null,
            frontDeskPhone: null,
            frontDeskHours: null,
            address: null,
            logo: { lightUrl: null, darkUrl: null },
            signIn: { google: true, password: true },
            joinCodes,
          }),
        ),
      );
      serveStudents([AN]);
      serveMembers();
      const { user } = renderStudents(teacherUser);

      const menu = await menuOf(user, "Nguyễn Văn An");
      await user.click(within(menu).getByRole("menuitem", { name: "Xoá khỏi lớp" }));
      const dialog = await screen.findByRole("dialog", {
        name: "Xoá Nguyễn Văn An khỏi các lớp của học viên?",
      });
      if (rejoin === null) {
        await waitFor(() => expect(dialog).not.toHaveTextContent("vào lại"));
        expect(dialog).toHaveTextContent("Tài khoản và kết quả vẫn được giữ.");
      } else {
        await waitFor(() => expect(dialog).toHaveTextContent(rejoin));
      }
    },
  );

  it("offers no removal for a student in no class", async () => {
    serveStudents([student("018f0000-0000-7000-8000-0000000000e9", "Đỗ Minh")]);
    const { user } = renderStudents(teacherUser);

    await rowOf("Đỗ Minh");
    const menu = await menuOf(user, "Đỗ Minh");
    expect(
      within(menu).getByRole("menuitem", { name: "Xoá khỏi lớp" }),
    ).toHaveAttribute("aria-disabled", "true");
  });
});

/**
 * An Admin's row lists every class the student is in, other teachers' too
 * (`scope.all`), and Remove and Move act on all of them, so each confirmation
 * names them (SR-40-1).
 */
describe("an Admin's Remove and Move name every class they end", () => {
  const OTHERS = { id: "018f0000-0000-7000-8000-0000000000a9", name: "TOEIC Cô Lan" };
  const WIDE = student("018f0000-0000-7000-8000-0000000000e1", "Nguyễn Văn An", {
    classes: [inClass(CLASS_A), inClass(OTHERS)],
  });
  const BINH = student("018f0000-0000-7000-8000-0000000000e2", "Trần Thị Bình", {
    classes: [inClass(OTHERS)],
  });

  it("names the classes in the row's Remove", async () => {
    serveStudents([WIDE]);
    const calls = serveMembers();
    const { user } = renderStudents(adminUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Xoá khỏi lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Xoá Nguyễn Văn An khỏi các lớp của học viên?",
    });
    expect(dialog).toHaveTextContent(
      `Nguyễn Văn An sẽ rời: ${CLASS_A.name}, ${OTHERS.name}.`,
    );
    await user.click(within(dialog).getByRole("button", { name: "Xoá khỏi lớp" }));
    await waitFor(() =>
      expect(calls).toEqual([
        `remove ${CLASS_A.id} ${WIDE.id}`,
        `remove ${OTHERS.id} ${WIDE.id}`,
      ]),
    );
  });

  it("names the classes a Move leaves, and none once they are kept", async () => {
    serveStudents([WIDE]);
    serveMembers();
    const { user } = renderStudents(adminUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Chuyển lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Chuyển Nguyễn Văn An" });
    await chooseClass(user, CLASS_B.name);
    expect(dialog).toHaveTextContent(
      `Nguyễn Văn An sẽ rời: ${CLASS_A.name}, ${OTHERS.name}.`,
    );
    await user.click(
      within(dialog).getByRole("switch", { name: "Giữ học viên ở cả lớp hiện tại" }),
    );
    expect(dialog).not.toHaveTextContent("sẽ rời");
  });

  it("counts the memberships in the bulk Remove and names their classes once each", async () => {
    serveStudents([WIDE, BINH]);
    serveMembers();
    const { user } = renderStudents(adminUser);

    for (const name of ["Nguyễn Văn An", "Trần Thị Bình"]) {
      await user.click(
        within(await rowOf(name)).getByRole("checkbox", { name: `Chọn ${name}` }),
      );
    }
    await user.click(screen.getByRole("button", { name: "Xoá khỏi lớp" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent(
      `(3 lượt tham gia): ${CLASS_A.name}, ${OTHERS.name}.`,
    );
  });
});
