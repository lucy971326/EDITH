import type { FileContent, FileEntry } from "../features/files/types";

const studioAPI = "http://127.0.0.1:8765";

type APIError = { message?: string };

async function errorMessage(response: Response) {
  const body = await response.json().catch((): APIError => ({}));
  return body.message ?? `读取项目文件失败（HTTP ${response.status}）`;
}

// ListChildren 读取一个相对目录的直接子项；空字符串表示项目根目录。
export async function ListChildren(relativeDir: string): Promise<FileEntry[]> {
  const query = new URLSearchParams({ path: relativeDir });
  const response = await fetch(`${studioAPI}/api/files?${query}`);
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }
  const body = await response.json() as { entries: FileEntry[] };
  return body.entries;
}

// ReadText 读取一个相对文件路径的文本内容。
export async function ReadText(relativeFile: string): Promise<FileContent> {
  const query = new URLSearchParams({ path: relativeFile });
  const response = await fetch(`${studioAPI}/api/files/content?${query}`);
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }
  return response.json() as Promise<FileContent>;
}
