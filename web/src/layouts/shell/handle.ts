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
 * TeacherHandle is what a route rebuilt for a console's shell declares as its
 * `handle`. `crumb` is the whole trail, ancestors first; `width: "full"`
 * lifts the page frame's maximum, the one exception the teacher's design
 * names (the import review), and a number gives the frame's maximum in pixels
 * where the Admin console's design draws one; `sidebar: "collapsed"` starts
 * the route with the sidebar collapsed and leaves the stored choice alone.
 * Only the leaf match's handle is read.
 */
export interface TeacherHandle {
  crumb: readonly TeacherCrumb[];
  width?: "full" | number;
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
