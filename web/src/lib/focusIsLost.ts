/**
 * focusIsLost reports whether keyboard focus has fallen to the document, as
 * it does when the focused element is removed or disabled, so that a caller
 * moves it deliberately only then and never takes it from where the user put
 * it.
 */
export function focusIsLost(): boolean {
  const active = document.activeElement;
  return active === null || active === document.body;
}
