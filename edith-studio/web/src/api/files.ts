import { apiRequest, readErrorMessage } from "./client";
import type { FileContent, FileEntry } from "../features/files/types";

// ListChildren 读取一个相对目录的直接子项；空字符串表示项目根目录。
export async function ListChildren(relativeDir: string): Promise<FileEntry[]> {
  const query = new URLSearchParams({ path: relativeDir });
  const response = await apiRequest(`/api/files?${query}`);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response, `读取项目文件失败（HTTP ${response.status}）`));
  }
  const body = await response.json() as { entries: FileEntry[] };
  return body.entries;
}

// ReadText 读取一个相对文件路径的文本内容。
export async function ReadText(relativeFile: string): Promise<FileContent> {
  const query = new URLSearchParams({ path: relativeFile });
  const response = await apiRequest(`/api/files/content?${query}`);
  if (!response.ok) {
    throw new Error(await readErrorMessage(response, `读取项目文件失败（HTTP ${response.status}）`));
  }
  return response.json() as Promise<FileContent>;
}
