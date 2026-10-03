// Extension point only. The starter ships no authentication: no guard, no user model, no endpoints.

/** Whoever a future provider authenticates. Deliberately opaque: add fields with the real provider. */
export interface AuthenticatedPrincipal {
  /** Opaque, stable identifier issued by the auth provider. */
  subjectId: string;
}

/** Contract a real authentication provider must fulfil. */
export interface AuthProvider {
  /** Resolves the caller behind a credential, or rejects when it is not valid. */
  authenticate(credential: string): Promise<AuthenticatedPrincipal>;
}

/** Injection token for the active AuthProvider. */
export const AUTH_PROVIDER = Symbol("AUTH_PROVIDER");
