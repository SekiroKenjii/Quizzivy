/** preventLocalEscapeDismissal keeps a dialog open while its focused input owns Escape. */
export function preventLocalEscapeDismissal(event: KeyboardEvent): void {
  const input = event.target;
  if (
    input instanceof HTMLInputElement &&
    input === input.ownerDocument.activeElement &&
    input.getAttribute("data-local-escape") === "true"
  ) {
    event.preventDefault();
  }
}
