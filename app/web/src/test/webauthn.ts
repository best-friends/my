// SPDX-License-Identifier: AGPL-3.0-only
// A browser with passkeys for the tests (lib/webauthn): jsdom has no WebAuthn, so these stand in for
// PublicKeyCredential, its two responses, and navigator.credentials. installPasskeys() puts them in the globals for
// one test (vitest.config.ts unstubs them afterwards); the credentials answer with fixed bytes.
import { vi } from "vitest";

const bytes = (...values: number[]) => new Uint8Array(values).buffer;

export class FakeAttestationResponse {
  readonly clientDataJSON = bytes(1, 2, 3);
  readonly attestationObject = bytes(4, 5, 6);
  getTransports(): string[] {
    return ["internal"];
  }
}

export class FakeAssertionResponse {
  readonly clientDataJSON = bytes(1, 2, 3);
  readonly authenticatorData = bytes(7, 8);
  readonly signature = bytes(9);
  readonly userHandle = bytes(0x75, 0x31);
}

export class FakePublicKeyCredential {
  readonly id = "AQID";
  readonly rawId = bytes(1, 2, 3);
  readonly type = "public-key";
  readonly authenticatorAttachment = "platform";
  readonly response: FakeAttestationResponse | FakeAssertionResponse;
  constructor(response: FakeAttestationResponse | FakeAssertionResponse) {
    this.response = response;
  }
  getClientExtensionResults(): Record<string, unknown> {
    return {};
  }
}

/** The prompts of the fake browser: each returns the credential, or rejects with what a test sets. */
export type FakeCredentials = {
  create: ReturnType<typeof vi.fn<(options: CredentialCreationOptions) => Promise<FakePublicKeyCredential>>>;
  get: ReturnType<typeof vi.fn<(options: CredentialRequestOptions) => Promise<FakePublicKeyCredential>>>;
};

/** Makes the browser of the test one with passkeys and returns its prompts. */
export function installPasskeys(): FakeCredentials {
  const credentials: FakeCredentials = {
    create: vi.fn(() => Promise.resolve(new FakePublicKeyCredential(new FakeAttestationResponse()))),
    get: vi.fn(() => Promise.resolve(new FakePublicKeyCredential(new FakeAssertionResponse()))),
  };
  vi.stubGlobal("PublicKeyCredential", FakePublicKeyCredential);
  vi.stubGlobal("AuthenticatorAttestationResponse", FakeAttestationResponse);
  vi.stubGlobal("AuthenticatorAssertionResponse", FakeAssertionResponse);
  Object.defineProperty(navigator, "credentials", { configurable: true, value: credentials });
  return credentials;
}

/** Takes the passkeys of installPasskeys() out of the browser again. */
export function removePasskeys(): void {
  Reflect.deleteProperty(navigator, "credentials");
}
