import { afterEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { registerSkeleton, skeletonFor } from "@/app/boot/handoff";
import { StudentArea } from "@/app/guards/RequireWorkspace";
import { homePathFor } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";
import {
  adminUser,
  adminWhoTakesTests,
  assistantUser,
  studentUser,
  teacherUser,
} from "@tests/support/fixtures";
import "@/lib/i18n";

const adminOnly: typeof adminUser = { ...adminUser, workspaces: ["admin"] };
const nobody: typeof studentUser = { ...studentUser, permissions: [], workspaces: [] };

function openStudentApp(user: typeof studentUser) {
  useAuthStore.setState({ isBootstrapping: false, accessToken: "t", user });
  const router = createMemoryRouter(
    [
      {
        path: "/app",
        element: <StudentArea />,
        children: [{ index: true, element: <p>trang học viên</p> }],
      },
      { path: "/teacher", element: <p>bàn làm việc của giáo viên</p> },
      { path: "*", element: <p>không có trang này</p> },
    ],
    { initialEntries: ["/before", "/app"], initialIndex: 1 },
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("where a signed-in user lands", () => {
  it.each([
    ["a teacher", teacherUser, "/teacher"],
    ["an assistant", assistantUser, "/teacher"],
    ["the admin", adminUser, "/teacher"],
    ["a user with the admin workspace alone", adminOnly, "/teacher"],
    ["an admin who takes tests", adminWhoTakesTests, "/teacher"],
    ["a student", studentUser, "/app"],
    ["a user with no workspace", nobody, "/app"],
    ["nobody", null, "/app"],
  ])("sends %s home to %s", (_who, user, home) => {
    expect(homePathFor(user)).toBe(home);
  });
});

describe("the student tree's guard", () => {
  it.each([
    ["a teacher", teacherUser],
    ["a user with the admin workspace alone", adminOnly],
  ])(
    "moves %s to /teacher, in place of the address they opened",
    async (_who, user) => {
      const router = openStudentApp(user);

      expect(await screen.findByText("bàn làm việc của giáo viên")).toBeInTheDocument();
      expect(router.state.location.pathname).toBe("/teacher");
      expect(router.state.historyAction).toBe("REPLACE");
      await router.navigate(-1);
      await waitFor(() => expect(router.state.location.pathname).toBe("/before"));
    },
  );

  it("leaves a student in the student app", async () => {
    const router = openStudentApp(studentUser);

    expect(await screen.findByText("trang học viên")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/app");
    expect(screen.queryByText("bàn làm việc của giáo viên")).not.toBeInTheDocument();
  });

  it("answers a user with no workspace with the 403, where they are", async () => {
    const router = openStudentApp(nobody);

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Bạn không có quyền mở trang này",
      }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/app");
    expect(screen.queryByText("trang học viên")).not.toBeInTheDocument();
  });
});

describe("the splash hand-off for the teacher's tree", () => {
  it("draws the teacher frame at /teacher and beneath it, and nowhere else", () => {
    expect(skeletonFor("/teacher")).toBeNull();

    registerSkeleton("teacher", () => <p>khung giáo viên</p>);

    for (const path of ["/teacher", "/teacher/", "/teacher/tests/abc/edit"]) {
      const { unmount } = render(<>{skeletonFor(path)}</>);
      expect(screen.getByText("khung giáo viên"), path).toBeInTheDocument();
      unmount();
    }
    for (const path of ["/teachers", "/teacher-old", "/app", "/login", "/"]) {
      expect(skeletonFor(path), path).toBeNull();
    }
  });
});
