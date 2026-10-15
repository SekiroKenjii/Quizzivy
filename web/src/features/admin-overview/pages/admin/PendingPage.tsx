import { useMatches } from "react-router";
import { Placeholder } from "@/components/shared/Placeholder";
import { adminHandleOf } from "@/app/guards/adminHandle";

/**
 * PendingPage stands for an Admin console screen that is not built yet. It is
 * titled by its route's last crumb, so a route keeps its place in the tree
 * until its own task gives it a page.
 */
export default function PendingPage() {
  const handle = adminHandleOf(useMatches().at(-1)?.handle);
  return <Placeholder titleKey={handle?.crumb.at(-1)?.key ?? "adminShell.crumb"} />;
}
