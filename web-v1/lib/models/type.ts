export type ProviderInfo = { id: string; name: string };
export type ReasoningOption = { id: string; name: string };
export type ModelCapabilities = { vision: boolean };
export type ModelInfo = {
  id: string;
  providerId: string;
  name: string;
  reasoningOptions: ReasoningOption[];
  capabilities: ModelCapabilities;
};
export type ModelCatalogResponse = { providers: ProviderInfo[]; models: ModelInfo[] };
export type AvailableModelCatalogResponse = { models: ModelInfo[] };
