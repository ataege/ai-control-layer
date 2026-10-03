import { Injectable, NotImplementedException } from "@nestjs/common";
import type { AuthenticatedPrincipal, AuthProvider } from "./auth.types.js";

/** Default binding for AUTH_PROVIDER. Fails loudly (501) instead of pretending anyone is authenticated. */
@Injectable()
export class UnimplementedAuthProvider implements AuthProvider {
  authenticate(): Promise<AuthenticatedPrincipal> {
    return Promise.reject(new NotImplementedException("Authentication is not implemented"));
  }
}
