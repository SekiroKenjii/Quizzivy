import { Navigate, useLocation } from "react-router";
import { assignmentStudentsLocation, legacyTeacherPath } from "./legacyTeacherPath";

/** LegacyTeacherRedirect replaces pre-R4 Teacher addresses with their canonical paths while preserving query values and hashes. */
export function LegacyTeacherRedirect() {
  const { pathname, search, hash } = useLocation();
  return (
    <Navigate
      replace
      to={
        assignmentStudentsLocation({ pathname, search, hash }) ?? {
          pathname: legacyTeacherPath(pathname) ?? "/teacher",
          search,
          hash,
        }
      }
    />
  );
}

/** AssignmentPapersRedirect replaces the retired paper page with the Students panel and its translated filter. */
export function AssignmentPapersRedirect() {
  const location = useLocation();
  return (
    <Navigate
      replace
      to={assignmentStudentsLocation(location) ?? "/teacher/assignments"}
    />
  );
}
