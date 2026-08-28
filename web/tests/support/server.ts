import { setupServer } from "msw/node";
import { handlers } from "./handlers";

/** Node-side MSW server. Started once in setup.ts. */
export const server = setupServer(...handlers);
