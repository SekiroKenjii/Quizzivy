import type { TFunction } from "i18next";
import { Ellipsis } from "lucide-react";
import type { DataColumn } from "@/components/shared/data/DataTable";
import type { AdminUser } from "@/features/users/api";
import {
  ClassesCell,
  LastActive,
  Person,
  RoleBadge,
  SignIn,
  StatusPill,
} from "@/features/users/components/UserCells";
import { lastActiveText } from "@/features/users/userText";
import type { Locale } from "@/lib/i18n";
import { roleName } from "@/lib/roleName";

/**
 * CARDS_BELOW is the content width under which each account is a card and not
 * a row, as the deck draws it.
 */
export const CARDS_BELOW = 560;

/**
 * userColumns are the table's columns as the deck draws them. The User column
 * always shows, with the email and, under it, the role and the last activity
 * while their columns are folded away. The others join by the content's width
 * (`showFrom`): Status from 560, Role from 640, Last active from 820, Classes
 * from 960 and Sign-in from 1080. The last column is the deck's ellipsis, a
 * pointer target that opens the row as the row does; the keyboard opens it
 * from the User column.
 */
export function userColumns(t: TFunction, locale: Locale): DataColumn<AdminUser>[] {
  return [
    {
      id: "user",
      header: t("users.column.user"),
      track: "minmax(200px,2.4fr)",
      cell: (user, shown) => {
        const sub = [
          user.email,
          shown.has("role") ? null : roleName(user.role, t),
          shown.has("last") ? null : lastActiveText(user, locale, t),
        ]
          .filter((part) => part !== null)
          .join(" · ");
        return <Person user={user} sub={sub} />;
      },
    },
    {
      id: "role",
      header: t("users.column.role"),
      track: "112px",
      showFrom: 640,
      cell: (user) => <RoleBadge role={user.role} />,
    },
    {
      id: "signIn",
      header: t("users.column.signIn"),
      track: "150px",
      showFrom: 1080,
      cell: (user) => <SignIn signIn={user.signIn} />,
    },
    {
      id: "classes",
      header: t("users.column.classes"),
      track: "minmax(130px,1.3fr)",
      showFrom: 960,
      cell: (user) => (
        <ClassesCell
          classes={user.classes}
          student={user.role.builtinKey === "student"}
        />
      ),
    },
    {
      id: "last",
      header: t("users.column.lastActive"),
      track: "110px",
      showFrom: 820,
      cell: (user) => (
        <span className="text-muted-fg text-[13px] whitespace-nowrap">
          <LastActive at={user.lastActiveAt} locale={locale} />
        </span>
      ),
    },
    {
      id: "status",
      header: t("users.column.status"),
      track: "minmax(96px,150px)",
      showFrom: 560,
      cell: (user, shown) => <StatusPill user={user} short={!shown.has("last")} />,
    },
    {
      id: "open",
      header: "",
      track: "40px",
      cell: () => (
        <span
          aria-hidden="true"
          className="rounded-seg hover:bg-hover grid size-7.5 place-items-center"
        >
          <Ellipsis className="size-4" />
        </span>
      ),
    },
  ];
}
