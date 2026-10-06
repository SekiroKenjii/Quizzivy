/** assignmentDetailLocation changes only owned query values while retaining the path, hash and unrelated parameters. */
export function assignmentDetailLocation(
  location: Readonly<{ pathname: string; search: string; hash: string }>,
  changes: Readonly<Record<string, string | null>>,
) {
  const query = new URLSearchParams(location.search);
  for (const [key, value] of Object.entries(changes)) {
    if (value === null || value === "") query.delete(key);
    else query.set(key, value);
  }
  const search = query.toString();
  return {
    pathname: location.pathname,
    search: search === "" ? "" : `?${search}`,
    hash: location.hash,
  };
}
