import { useEffect } from "react";
import type { DataRouter } from "react-router";
import { AppStateLayer } from "./boot/AppStateLayer";
import { BootSplash } from "./boot/BootSplash";
import { trackActivity } from "./boot/activity";
import { useVersionWatch } from "./boot/version";

/**
 * AppFrame is everything that stands over the routes: the boot splash, the
 * overlay layer, and the watch for a newer build. It renders beside the
 * router rather than inside it, so it is on screen while a deep link's lazy
 * route code is still loading and survives a route that fails to load.
 */
export function AppFrame({ router }: Readonly<{ router: DataRouter }>) {
  useEffect(() => trackActivity(), []);
  useVersionWatch(router);
  return (
    <>
      <BootSplash router={router} />
      <AppStateLayer router={router} />
    </>
  );
}
