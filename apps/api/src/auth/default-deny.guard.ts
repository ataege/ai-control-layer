import {
  CanActivate,
  ExecutionContext,
  Inject,
  Injectable,
  UnauthorizedException,
  ServiceUnavailableException,
} from "@nestjs/common";
import { Reflector } from "@nestjs/core";
import { IS_PUBLIC_KEY } from "./public.decorator.js";
import type { Request } from "express";
import { AUTH_PROVIDER, AuthProvider, type AuthenticatedPrincipal } from "./auth.types.js";
import { readSessionCookie } from "./session-cookie.js";
import { DataSource, Repository } from "typeorm";
import { InjectRepository } from "@nestjs/typeorm";
import { Membership } from "../identity/entities/membership.entity.js";
import { OperatorContext } from "@workspace/contracts";

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      operatorContext: OperatorContext;
      /** The principal the auth provider resolved from the session cookie. */
      user?: AuthenticatedPrincipal;
    }
  }
}

/**
 * Global guard that denies access to all routes unless they are marked with @Public().
 * Protected routes require a verified stored session and a current trusted membership.
 */
@Injectable()
export class DefaultDenyGuard implements CanActivate {
  constructor(
    private reflector: Reflector,
    @Inject(AUTH_PROVIDER) private authProvider: AuthProvider,
    private dataSource: DataSource,
    @InjectRepository(Membership) private membershipRepository: Repository<Membership>,
  ) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const isPublic = this.reflector.getAllAndOverride<boolean>(IS_PUBLIC_KEY, [
      context.getHandler(),
      context.getClass(),
    ]);

    if (isPublic) {
      return true;
    }

    if (!this.dataSource.isInitialized) {
      throw new ServiceUnavailableException("Database is not ready");
    }

    const request = context.switchToHttp().getRequest<Request>();
    const sessionCookie = readSessionCookie(request);

    if (!sessionCookie) {
      throw new UnauthorizedException("Missing session cookie");
    }

    let principal;
    try {
      principal = await this.authProvider.authenticate(sessionCookie);
    } catch (error) {
      if (error instanceof UnauthorizedException) {
        throw new UnauthorizedException("Invalid or expired session");
      }
      throw new ServiceUnavailableException("Authentication unavailable");
    }

    // Known limit: one organization per operator in the demo, with no organization switch. The
    // membership table allows a user in several organizations; the oldest membership decides.
    let membership;
    try {
      membership = await this.membershipRepository.findOne({
        where: { userId: principal.subjectId },
        order: { createdAt: "ASC" },
      });
    } catch {
      throw new ServiceUnavailableException("Database unavailable");
    }

    if (!membership) {
      throw new UnauthorizedException("User has no organization membership");
    }

    request.operatorContext = {
      userId: membership.userId,
      organizationId: membership.organizationId,
      roles: membership.roles,
    };

    request.user = principal;
    return true;
  }
}
