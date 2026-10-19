import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { DataTable } from "@/components/shared/data/DataTable";
import { Pager } from "@/components/shared/data/Pager";
import type { BulkSelection } from "@/components/shared/BulkSelection";
import { USER_PAGE_SIZES, type AdminUser } from "@/features/users/api";
import { UserCard } from "@/features/users/components/UserCard";
import { userColumns } from "@/features/users/components/userColumns";
import type { Locale } from "@/lib/i18n";

/**
 * UsersTable is a page of accounts: a table whose columns join as the content
 * widens, or a list of cards when `cards` says the content is narrower than
 * CARDS_BELOW, and under it the pager. `empty` is what an empty page says.
 */
export function UsersTable({
  users,
  page,
  pageSize,
  total,
  cards,
  empty,
  locale,
  onOpen,
  selection,
}: Readonly<{
  users: readonly AdminUser[];
  page: number;
  pageSize: number;
  total: number;
  cards: boolean;
  empty: ReactNode;
  locale: Locale;
  onOpen: (user: AdminUser) => void;
  selection: BulkSelection<AdminUser>;
}>) {
  const { t } = useTranslation();
  const columns = useMemo(() => userColumns(t, locale), [t, locale]);
  return (
    <DataTable
      label={t("users.table")}
      columns={columns}
      rows={users}
      rowSize={{ height: 58 }}
      card={(user) => <UserCard user={user} locale={locale} />}
      cardLayout="joined"
      cards={cards}
      empty={empty}
      onOpen={onOpen}
      selection={selection}
      rowName={(user) => user.fullName}
      footer={
        <Pager
          page={page}
          pageSize={pageSize}
          total={total}
          sizes={USER_PAGE_SIZES}
          noun={(count) => t("users.noun", { count })}
        />
      }
    />
  );
}
