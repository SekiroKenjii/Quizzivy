import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { QueryStates } from "@/components/shared/ListState";
import { PageHead } from "@/layouts/shell/PageHead";
import { rolesQuery } from "@/features/roles/api";
import { RolesBoard } from "@/features/roles/components/RolesBoard";
import { RolesSkeleton } from "@/features/roles/components/RolesSkeleton";

/**
 * RolesPage is the Admin console's Roles & permissions screen (T-R5.24): the
 * roles as cards in a carousel, and under them the matrix of what each may
 * do, edited in place and saved as one.
 */
export default function RolesPage() {
  const { t } = useTranslation();
  const query = useQuery(rolesQuery());
  return (
    <div className="flex flex-col gap-4">
      <PageHead title={t("roles.title")} description={t("roles.lead")} />
      <QueryStates
        query={query}
        skeleton={<RolesSkeleton />}
        failed={t("roles.loadFailed")}
      >
        {(list) => <RolesBoard list={list} />}
      </QueryStates>
    </div>
  );
}
