/** The rules a new password must meet, as `changePassword` in api/openapi.yaml states them. */

/** MIN_PASSWORD_LENGTH counts characters, not UTF-16 units, as the server does. */
export const MIN_PASSWORD_LENGTH = 8;

const NUMBER_OR_SYMBOL = /[\p{N}\p{P}\p{S}]/u;
const LONG_PASSWORD = 12;

/** PasswordRules says which of the two client-checkable rules a password meets. */
export type PasswordRules = { length: boolean; numberOrSymbol: boolean };

/** passwordRules checks the length and the number-or-symbol rules. */
export function passwordRules(password: string): PasswordRules {
  return {
    length: [...password].length >= MIN_PASSWORD_LENGTH,
    numberOrSymbol: NUMBER_OR_SYMBOL.test(password),
  };
}

/**
 * passwordStrength scores a password from 0 to 4 for the deck's meter: one
 * for each rule met, one for any password at all (the browser cannot see the
 * temporary password, so it cannot fail that rule), and one at twelve
 * characters or more.
 */
export function passwordStrength(password: string): number {
  if (password === "") return 0;
  const rules = passwordRules(password);
  return (
    1 +
    Number(rules.length) +
    Number(rules.numberOrSymbol) +
    Number([...password].length >= LONG_PASSWORD)
  );
}
