import { describe, expect, it } from "vitest";
import en from "@/lib/i18n/locales/en.json";
import vi from "@/lib/i18n/locales/vi.json";

const WRITTEN = [
  "ai_credit_periods",
  "ai_credit_requests",
  "ai_settings",
  "ai_teacher_credits",
  "assignment",
  "attempt",
  "attempt_events",
  "audit_log",
  "class",
  "class_join_code",
  "class_member",
  "data_export",
  "maintenance_window",
  "media_asset",
  "organization",
  "question",
  "question_group",
  "refresh_token_family",
  "role",
  "student",
  "test",
  "test_version",
  "user",
  "user_identity",
  "user_import",
  "users",
  "word_import",
];

describe("the audit log's entity types", () => {
  it.each([
    ["vi", vi.audit.entity.types as Record<string, string>],
    ["en", en.audit.entity.types as Record<string, string>],
  ])("%s names every type the server writes", (_, types) => {
    expect(WRITTEN.filter((type) => !types[type]?.trim())).toEqual([]);
  });
});
