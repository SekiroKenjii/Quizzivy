/** FIELD_GRID flows the fields of a card into at most three columns of at least 260px (DG-205). */
export const FIELD_GRID =
  "grid grid-cols-[repeat(auto-fit,minmax(max(min(260px,100%),calc((100%_-_2.5rem)/3)),1fr))] gap-x-5 gap-y-3.5";

/** FIELD_INPUT is the deck's 38px input on the page ground. */
export const FIELD_INPUT = "bg-bg h-9.5";

/** describedBy is the `aria-describedby` of a SettingsField's control. */
export function describedBy(id: string, hint: boolean, error: boolean) {
  if (error) return `${id}-error`;
  return hint ? `${id}-hint` : undefined;
}

/** controlId is the id of the control that edits `path`, a field of the settings form. */
export function controlId(path: string) {
  return `system-${path.replaceAll(".", "-")}`;
}
