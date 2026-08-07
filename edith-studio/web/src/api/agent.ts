import { apiRequest } from "./client";

export type RunImage = { name: string; dataUrl: string };

export type RunRequest = {
  requestId: string;
  sessionId: string;
  message: string;
  modelId: string;
  thinkingMode: string;
  images?: RunImage[];
};

// startRun 返回原始 Response；调用方通过 SSE 读取流式回复。
export async function startRun(request: RunRequest) {
  return apiRequest("/api/runs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}

export async function cancelRun(requestID: string) {
  return apiRequest(`/api/runs/${requestID}/cancel`, { method: "POST" });
}
