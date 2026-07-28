export type ProviderCredentialState = { providerId: string; hasApiKey: boolean };
export type UserSettingsResponse = { personality: string; providers: ProviderCredentialState[] };
export type ProviderCredentialInput = { providerId: string; apiKey?: string };
export type SaveUserSettingsRequest = { personality: string; providers: ProviderCredentialInput[] };
