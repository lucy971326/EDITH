const studioAPI = "http://127.0.0.1:8765";

export type ThinkingProfile = {
  defaultMode: string;
  modes: string[];
};

export type ModelProfile = {
  id: string;
  provider: string;
  name: string;
  contextWindow: number;
  vision: boolean;
  thinking: ThinkingProfile;
};

export type ModelCatalog = {
  defaultModelId: string;
  models: ModelProfile[];
};

export async function getModels(): Promise<ModelCatalog> {
  const response = await fetch(`${studioAPI}/api/models`);
  if (!response.ok) {
    throw new Error("无法读取模型目录");
  }
  return response.json() as Promise<ModelCatalog>;
}
