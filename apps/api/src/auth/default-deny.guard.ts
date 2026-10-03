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
import { AUTH_PROVIDER, AuthProvider } from "./auth.types.js";
import { DataSource, Repository } from "typeorm";
import { InjectRepository } from "@nestjs/typeorm";
import { Membership } from "../identity/entities/membership.entity.js";
import { OperatorContext } from "@workspace/contracts";

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      operatorContext: OperatorContext;
    }
  }
}

/**
 * Global guard that denies access to all routes unless they are marked with @Public().
 * As per decision 7 hold, protected routes return 501 Not Implemented.
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
    const sessionCookie = request.cookies?.session;

    if (!sessionCookie) {
      throw new UnauthorizedException("Missing session cookie");
    }

    let principal;
    try {
      principal = await this.authProvider.authenticate(sessionCookie);
    } catch {
      throw new UnauthorizedException("Invalid or expired session");
    }

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

    (request as any)["user"] = principal;
    return true;
  }
}
