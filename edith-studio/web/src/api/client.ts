// client 是前端访问本地 Studio Go 服务的统一入口，集中 base URL 与请求辅助函数。

const studioAPI = "http://127.0.0.1:8765";

type APIErrorBody = { message?: string };

// apiRequest 直接请求本地 Studio 并返回原始 Response；流式接口需要保留 body。
export function apiRequest(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${studioAPI}${path}`, init);
}

// apiJSON 请求 JSON 接口，非 2xx 时优先使用后端返回的 message 抛错。
export async function apiJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await apiRequest(path, init);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response, `请求失败（HTTP ${response.status}）`));
  }
  return response.json() as Promise<T>;
}

// readErrorMessage 优先使用后端返回的 message；没有时使用调用方提供的回退文案。
export async function readErrorMessage(response: Response, fallback: string): Promise<string> {
  const body = await response.json().catch((): APIErrorBody | null => null);
  return body?.message ?? fallback;
}
