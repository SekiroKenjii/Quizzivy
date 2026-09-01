import { api } from "./client";

/**
 * Compile-time assertions that the client is genuinely constrained by the
 * generated contract. There is nothing to run: `tsc -b` is the test.
 *
 * `@ts-expect-error` is self-verifying — if the line below it ever stops being
 * an error, TypeScript reports the directive as unused and the build fails. So
 * these cannot rot into decoration.
 */

export async function _valid() {
  // Path params are required and named.
  const test = await api("get", "/admin/tests/{id}", { path: { id: "abc" } });
  const _title: string = test.title;
  void _title;

  // Response shape flows through.
  const me = await api("get", "/auth/me");
  const _role: "admin" | "student" = me.role;
  void _role;

  // Request bodies are checked.
  await api("post", "/auth/login", { body: { email: "a@b.c", password: "hunter22" } });

  // Query params are optional but typed.
  await api("get", "/admin/questions", { query: { type: ["short_answer"] } });

  // 204 endpoints resolve to void.
  const nothing: void = await api("post", "/auth/logout");
  return nothing;
}

export async function _invalid() {
  // @ts-expect-error — no such path in the contract
  await api("get", "/admin/not-a-real-endpoint");

  // @ts-expect-error — /auth/me has no POST
  await api("post", "/auth/me");

  // @ts-expect-error — password is required by the login body
  await api("post", "/auth/login", { body: { email: "a@b.c" } });

  // @ts-expect-error — `id` is the declared path param, not `testId`
  await api("get", "/admin/tests/{id}", { path: { testId: "abc" } });

  // @ts-expect-error — question type is an enum; "essay" is not in it
  await api("get", "/admin/questions", { query: { type: "essay" } });
}

/**
 * The security boundary, at the type level. §13.5 forbids these keys in a
 * student payload, and the generated types already omit them — so reading one
 * off the take-test response must not compile.
 */
export async function _studentPayloadHasNoGradingKey() {
  const session = await api("get", "/app/attempts/{id}", { path: { id: "a" } });
  const question = session.questions[0]!;

  const _prompt: string = question.prompt;
  void _prompt;

  // @ts-expect-error — §13.5: a student question carries no sampleAnswer
  void question.sampleAnswer;

  // @ts-expect-error — §13.5: nor a transcript
  void question.transcript;

  // @ts-expect-error — §13.5: nor the correct-option flags
  void question.options?.[0]?.isCorrect;
}
