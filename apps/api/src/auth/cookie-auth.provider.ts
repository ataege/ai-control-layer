import { Injectable, UnauthorizedException } from "@nestjs/common";
import { InjectRepository } from "@nestjs/typeorm";
import { Repository } from "typeorm";
import { Session } from "../identity/entities/session.entity.js";
import { createHash } from "node:crypto";
import { AuthProvider, AuthenticatedPrincipal } from "./auth.types.js";

@Injectable()
export class CookieAuthProvider implements AuthProvider {
  constructor(
    @InjectRepository(Session)
    private readonly sessionRepository: Repository<Session>,
  ) {}

  async authenticate(credential: string): Promise<AuthenticatedPrincipal> {
    const session = await this.sessionRepository.findOne({
      where: { id: createHash("sha256").update(credential).digest("hex") },
    });

    if (!session) {
      throw new UnauthorizedException("Session not found");
    }

    if (session.expiresAt < new Date()) {
      throw new UnauthorizedException("Session expired");
    }

    return {
      subjectId: session.userId,
    };
  }
}
