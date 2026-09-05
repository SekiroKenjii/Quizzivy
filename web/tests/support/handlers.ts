import { http } from "msw";
import { contractJson } from "./contractResponse";
import { adminUser, myClass, sampleClass, studentUser } from "./fixtures";

const BASE = "http://localhost:8080";

/**
 * Default handlers: the happy path, enough for a component to mount. Tests
 * override per-case with `server.use(...)`.
 */
export const handlers = [
  http.get(`${BASE}/auth/me`, () => contractJson("/auth/me", "get", 200, studentUser)),

  http.post(`${BASE}/auth/login`, () =>
    contractJson("/auth/login", "post", 200, {
      accessToken: "test-access-token",
      expiresIn: 900,
      user: adminUser,
    }),
  ),

  http.post(`${BASE}/auth/refresh`, () =>
    contractJson("/auth/refresh", "post", 200, {
      accessToken: "test-refreshed-token",
      expiresIn: 900,
    }),
  ),

  http.post(`${BASE}/join/preview`, () =>
    contractJson("/join/preview", "post", 200, {
      classId: sampleClass.id,
      className: sampleClass.name,
      teacherName: "Thuong",
    }),
  ),

  http.get(`${BASE}/app/classes`, () =>
    contractJson("/app/classes", "get", 200, { items: [myClass] }),
  ),

  http.get(`${BASE}/app/assignments`, () =>
    contractJson("/app/assignments", "get", 200, {
      dueNow: [],
      upcoming: [],
      completed: [],
    }),
  ),
];
