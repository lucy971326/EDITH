export type FileEntry = {
  path: string;
  name: string;
  kind: "file" | "directory";
};

export type FileContent = {
  path: string;
  language: string;
  content: string;
  truncated: boolean;
};
