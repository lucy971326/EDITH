import type { ProviderCredentialInput, SaveUserSettingsRequest } from "./type";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// parseSaveUserSettingsRequest is the browser-facing settings boundary before
// Clerk's userId is added by the BFF route.
export function parseSaveUserSettingsRequest(value: unknown): SaveUserSettingsRequest | null {
  if (!isRecord(value) || typeof value.personality !== "string" || typeof value.defaultModelId !== "string" || !value.defaultModelId.trim() || !Array.isArray(value.providers)) {
    return null;
  }

  const providers: ProviderCredentialInput[] = [];
  for (const provider of value.providers) {
    if (!isRecord(provider) || typeof provider.providerId !== "string" || !provider.providerId.trim()) {
      return null;
    }
    if ("apiKey" in provider && typeof provider.apiKey !== "string") {
      return null;
    }
    providers.push({
      providerId: provider.providerId.trim(),
      ...(typeof provider.apiKey === "string" ? { apiKey: provider.apiKey } : {}),
    });
  }
  return {
    personality: value.personality,
    defaultModelId: value.defaultModelId.trim(),
    ...(typeof value.timezone === "string" ? { timezone: value.timezone.trim() } : {}),
    providers,
  };
}

