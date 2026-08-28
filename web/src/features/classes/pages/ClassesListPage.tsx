import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { api } from "@/lib/api/client";

/**
 * The teacher's classes. Deliberately thin: it exists so the §6.4 panel is
 * reachable, and creating and editing classes is Phase 2 work.
 */
export default function ClassesListPage() {
  const { t } = useTranslation();
  const classes = useQuery({
    queryKey: ["admin-classes"],
    queryFn: ({ signal }) => api("get", "/admin/classes", { signal }),
  });

  if (classes.isPending) {
    return (
      <p className="text-muted-foreground text-sm" role="status" aria-live="polite">
        {t("common.loading")}
      </p>
    );
  }
  if (classes.isError) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {t("classes.loadFailed")}
      </p>
    );
  }

  const items = classes.data.items;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t("nav.classes")}</h1>

      {items.length === 0 ? (
        <p className="text-muted-foreground text-sm">{t("classes.empty")}</p>
      ) : (
        <ul className="divide-y rounded-lg border">
          {items.map((c) => (
            <li key={c.id}>
              <Link
                to={`/admin/classes/${c.id}`}
                className="hover:bg-secondary focus-visible:ring-ring flex items-center justify-between px-6 py-4 transition-colors focus-visible:ring-2 focus-visible:outline-none"
              >
                <span className="font-medium">{c.name}</span>
                <span className="text-muted-foreground text-sm">
                  {t("classes.studentCount", { count: c.studentCount })}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
