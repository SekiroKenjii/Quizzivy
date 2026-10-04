import { Navigate, useLocation } from "react-router";
import { legacyTeacherPath } from "./legacyTeacherPath";

/**
 * LegacyTeacherRedirect is the route element for `/admin` and everything
 * beneath it: it replaces the address with the same path under `/teacher`,
 * keeping the query and the hash. It stays until R5 gives `/admin` to the
 * Admin console.
 */
export function LegacyTeacherRedirect() {
  const { pathname, search, hash } = useLocation();
  return (
    <Navigate
      replace
      to={{ pathname: legacyTeacherPath(pathname) ?? "/teacher", search, hash }}
    />
  );
}
