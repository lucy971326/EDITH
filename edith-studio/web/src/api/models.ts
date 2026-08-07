import { apiJSON } from "./client";

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
  return apiJSON<ModelCatalog>("/api/models");
}
