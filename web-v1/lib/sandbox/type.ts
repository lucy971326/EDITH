// Sandbox 文件接口只描述浏览器可见的数据；用户身份由 Next BFF 从 Clerk 注入。
export type SandboxEntryType = "file" | "directory";

export type SandboxFileEntry = {
  name: string;
  path: string;
  type: SandboxEntryType;
  size?: number;
};

export type SandboxFilesResponse = {
  path: string;
  entries: SandboxFileEntry[];
};

export type SandboxFileContentResponse = {
  path: string;
  content: string;
  truncated: boolean;
};
