// 普通文件仅在发送时上传；返回后端分配的 uploads/ 相对路径。
export async function uploadSandboxFile(sessionId: string, file: File): Promise<string> {
  if (file.size <= 0 || file.size > 50 * 1024 * 1024) throw new Error("文件必须非空且不超过 50MB");
  const body = new FormData(); body.append("file", file);
  const response = await fetch(`/api/sandbox/files/upload?sessionId=${encodeURIComponent(sessionId)}`, { method: "POST", body });
  if (!response.ok) throw new Error((await response.json().catch(() => null) as { message?: string } | null)?.message || "文件上传失败");
  const value = await response.json() as { path: string };
  return value.path;
}
