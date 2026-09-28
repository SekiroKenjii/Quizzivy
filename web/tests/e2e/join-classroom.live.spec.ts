import { expect, test, type APIRequestContext } from "@playwright/test";
import { ADMIN } from "./support/live";

const API = "http://localhost:8080";

const classroom = Array.from(
  { length: 30 },
  (_, i) => `hocvien.lop${String(i + 1).padStart(2, "0")}@quizzivy.com`,
);

async function signIn(api: APIRequestContext, email: string, password: string) {
  const response = await api.post(`${API}/auth/login`, { data: { email, password } });
  expect(response.status(), `sign-in of ${email}`).toBe(200);
  return ((await response.json()) as { accessToken: string }).accessToken;
}

test("thirty students behind one address sign in, preview and join without a 429", async ({
  playwright,
}) => {
  test.setTimeout(180_000);
  const teacherApi = await playwright.request.newContext();
  const teacher = await signIn(teacherApi, ADMIN.email, ADMIN.password);
  const auth = { Authorization: `Bearer ${teacher}` };

  const created = await teacherApi.post(`${API}/teacher/classes`, {
    headers: auth,
    data: { name: `Lớp cả trường ${Date.now()}` },
  });
  expect(created.status()).toBe(201);
  const classId = ((await created.json()) as { id: string }).id;
  const issued = await teacherApi.post(`${API}/teacher/classes/${classId}/join-code`, {
    headers: auth,
    data: {},
  });
  expect(issued.status()).toBe(201);
  const joinCode = ((await issued.json()) as { code: string }).code;

  const statuses = await Promise.all(
    classroom.map(async (email) => {
      const api = await playwright.request.newContext();
      try {
        const token = await signIn(api, email, "quizzivy-dev");
        const preview = await api.post(`${API}/join/preview`, { data: { joinCode } });
        const joined = await api.post(`${API}/app/classes/join`, {
          headers: { Authorization: `Bearer ${token}` },
          data: { joinCode },
        });
        return [preview.status(), joined.status()];
      } finally {
        await api.dispose();
      }
    }),
  );
  for (const [i, [preview, joined]] of statuses.entries()) {
    expect(preview, `${classroom[i]} preview`).toBe(200);
    expect(joined, `${classroom[i]} join`).toBe(200);
  }

  const members = await teacherApi.get(
    `${API}/teacher/classes/${classId}/members?limit=100`,
    {
      headers: auth,
    },
  );
  expect(members.status()).toBe(200);
  const listed = (await members.json()) as { items: { email: string }[] };
  expect(new Set(listed.items.map((m) => m.email))).toEqual(new Set(classroom));
  await teacherApi.dispose();
});
