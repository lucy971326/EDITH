const agentAPI = "http://127.0.0.1:8765";

export type RunImage = { name: string; dataUrl: string };

export type RunRequest = {
  requestId: string;
  sessionId: string;
  message: string;
  modelId: string;
  thinkingMode: string;
  images?: RunImage[];
};

export async function startRun(request: RunRequest) {
  return fetch(`${agentAPI}/api/runs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
}

export async function cancelRun(requestID: string) {
  return fetch(`${agentAPI}/api/runs/${requestID}/cancel`, { method: "POST" });
}
