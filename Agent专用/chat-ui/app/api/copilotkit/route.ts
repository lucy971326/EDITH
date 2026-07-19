import {
  CopilotRuntime,
  createCopilotHonoHandler,
} from "@copilotkit/runtime/v2";
import { handle } from "hono/vercel";
import { HttpAgent } from "@ag-ui/client";

const aguiAgent = new HttpAgent({
  agentId: "agui-demo",
  url: process.env.AG_UI_ENDPOINT ?? "http://127.0.0.1:8080/agui",
});

const runtime = new CopilotRuntime({
  agents: {
    "agui-demo": aguiAgent as any,
  },
});

const app = createCopilotHonoHandler({
  runtime,
  basePath: "/api/copilotkit",
  mode: "single-route", // 配 useSingleEndpoint
});

export const GET = handle(app);
export const POST = handle(app);
export const OPTIONS = handle(app);
