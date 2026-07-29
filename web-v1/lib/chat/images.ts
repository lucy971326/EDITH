import type { ChatImage, CreateImageUploadResponse } from "./type";

export const acceptedImageTypes = ["image/jpeg", "image/png", "image/webp"] as const;
export const maxImageBytes = 10 * 1024 * 1024;

export async function uploadChatImage(sessionId: string, file: File): Promise<ChatImage> {
  const response = await fetch("/api/images", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sessionId, mimeType: file.type, sizeBytes: file.size }),
  });
  if (!response.ok) throw new Error(await response.text());

  const upload = await response.json() as CreateImageUploadResponse;
  const putResponse = await fetch(upload.uploadUrl, {
    method: "PUT",
    headers: { "Content-Type": file.type },
    body: file,
  });
  if (!putResponse.ok) throw new Error("图片上传到存储服务失败");

  const completeResponse = await fetch(`/api/images/${encodeURIComponent(upload.image.id)}/complete`, {
    method: "POST",
  });
  if (!completeResponse.ok) throw new Error(await completeResponse.text());
  const completed = await completeResponse.json() as { image: ChatImage };
  return completed.image;
}

export function validateImageFile(file: File): string | null {
  if (!acceptedImageTypes.includes(file.type as typeof acceptedImageTypes[number])) {
    return "只支持 JPEG、PNG 和 WebP 图片";
  }
  if (file.size > maxImageBytes) return "单张图片不能超过 10 MB";
  if (file.size === 0) return "不能上传空图片";
  return null;
}
