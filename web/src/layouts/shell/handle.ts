/**
 * TeacherCrumb is one entry of a route's breadcrumb trail: `key` is the locale
 * key of its label and `to` the address an ancestor links to. The last entry
 * of a trail is the page itself and has no `to`.
 */
export interface TeacherCrumb {
  key: string;
  to?: string;
}

/**
 * TeacherHandle is what a route rebuilt for the teacher shell declares as its
 * `handle`. `crumb` is the whole trail, ancestors first; `sidebar:
 * "collapsed"` starts the route with the sidebar collapsed and leaves the
 * stored choice alone. Only the leaf match's handle is read. A route declares
 * no width, since every page shares the shell's frame.
 */
export interface TeacherHandle {
  crumb: readonly TeacherCrumb[];
  sidebar?: "collapsed";
}

/**
 * teacherHandleOf returns `handle` when it is a teacher handle, an object
 * whose `crumb` is an array, and null for anything else.
 */
export function teacherHandleOf(handle: unknown): TeacherHandle | null {
  if (typeof handle !== "object" || handle === null) return null;
  if (!("crumb" in handle) || !Array.isArray(handle.crumb)) return null;
  return handle as TeacherHandle;
}
