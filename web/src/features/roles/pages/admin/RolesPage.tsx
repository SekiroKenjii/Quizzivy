import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { QueryStates } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { useCan } from "@/features/auth/permissions";
import { rolesQuery } from "@/features/roles/api";
import { RolesBoard } from "@/features/roles/components/RolesBoard";
import { RolesSkeleton } from "@/features/roles/components/RolesSkeleton";
import { PageHead } from "@/layouts/shell/PageHead";

/**
 * RolesPage is the Admin console's Roles & permissions screen (T-R5.24): the
 * roles as cards in a carousel, and under them the matrix of what each may
 * do, edited in place and saved as one. "New role" is offered to whoever
 * holds `people.roles.manage`, once the roles have loaded.
 */
export default function RolesPage() {
  const { t } = useTranslation();
  const query = useQuery(rolesQuery());
  const canManage = useCan("people.roles.manage");
  const [creating, setCreating] = useState(false);
  const newButton = useRef<HTMLButtonElement>(null);
  return (
    <div className="flex flex-col gap-4">
      <PageHead
        title={t("roles.title")}
        description={t("roles.lead")}
        actions={
          canManage && query.data !== undefined ? (
            <Button ref={newButton} variant="outline" onClick={() => setCreating(true)}>
              <Plus aria-hidden="true" />
              {t("roles.newRole.title")}
            </Button>
          ) : undefined
        }
      />
      <QueryStates
        query={query}
        skeleton={<RolesSkeleton />}
        failed={t("roles.loadFailed")}
      >
        {(list) => (
          <RolesBoard
            list={list}
            creating={creating}
            onCreatingChange={setCreating}
            newButton={newButton}
          />
        )}
      </QueryStates>
    </div>
  );
}
