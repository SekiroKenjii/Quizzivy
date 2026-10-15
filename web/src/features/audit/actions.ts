import type { AuditCategory } from "./api";

/**
 * humanise words an action key the client has no sentence for as the server
 * does, so an entry written by a newer server stays readable: the key with
 * its dots and underscores read as spaces.
 */
export function humanise(key: string): string {
  return key.replaceAll(".", " ").replaceAll("_", " ");
}

/**
 * sentenceOf is the sentence that words an action: `sentences` is the
 * locale's `audit.action` table, whose keys are the action keys themselves
 * (they hold dots, so `t()` cannot reach them one by one), and an action the
 * table lacks is humanised.
 */
export function sentenceOf(
  sentences: Readonly<Record<string, string>>,
  action: string,
): string {
  return sentences[action] ?? humanise(action);
}

const NAMED_ENTITIES: ReadonlySet<string> = new Set([
  "user",
  "users",
  "student",
  "user_identity",
  "ai_teacher_credits",
  "class",
  "class_member",
  "class_join_code",
  "test",
  "test_version",
  "assignment",
  "attempt",
  "media_asset",
  "word_import",
  "question_group",
  "ai_credit_requests",
]);

/**
 * isGone reports whether an entity the server sent with an id and no label
 * is a row that no longer exists. The server names only the kinds of row in
 * `NAMED_ENTITIES` (`AuditEntity.label` lists them), so a missing label on
 * any other kind, a role or a question, says nothing about the row.
 */
export function isGone(entity: Readonly<{ type: string; id: string | null }>): boolean {
  return entity.id !== null && NAMED_ENTITIES.has(entity.type);
}

const DOTS: Readonly<Record<AuditCategory, string>> = {
  accounts: "bg-danger",
  classes: "bg-info",
  attempts: "bg-warning",
  tests: "bg-success",
  media: "bg-muted-fg",
  imports: "bg-warning",
};

/** dotOf is the class of the small dot before an entry's sentence: the deck's colour for its category, and a neutral one for an entry in none. */
export function dotOf(category: AuditCategory | null): string {
  return category === null ? "bg-muted-fg" : DOTS[category];
}
