import {
  Controller,
  Post,
  Body,
  Res,
  Req,
  HttpCode,
  UnauthorizedException,
  UsePipes,
  Get,
  Header,
  ServiceUnavailableException,
} from "@nestjs/common";
import type { Response, Request } from "express";
import { InjectRepository } from "@nestjs/typeorm";
import { Repository, LessThan } from "typeorm";
import { randomUUID, createHash } from "node:crypto";
import { z } from "zod";
import { Public } from "./public.decorator.js";
import { User } from "../identity/entities/user.entity.js";
import { PasswordHash } from "../identity/entities/password-hash.entity.js";
import { Session } from "../identity/entities/session.entity.js";
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from "@nestjs/swagger";
import { verifyPassword } from "./password.util.js";
import { ZodValidationPipe } from "../common/zod-validation.pipe.js";
import { readSessionCookie } from "./session-cookie.js";

const signInSchema = z.object({
  email: z.string().email(),
  password: z.string().max(256),
});

type SignInBody = z.infer<typeof signInSchema>;

const DUMMY_HASH =
  "s$203ae1bfb6f75447afa7e0250281a922$53fa0c8e8b2d87c05550f51a4646ddb845fc9034a42d7149bdf5d2c27db100cfff2cb595b965393b151494cbff873e4fa6617c6d997453f1a2a90a494ffa3188";

function hashSessionId(sessionId: string): string {
  return createHash("sha256").update(sessionId).digest("hex");
}

@ApiTags("Auth")
@Controller("auth")
export class AuthController {
  constructor(
    @InjectRepository(User) private readonly userRepository: Repository<User>,
    @InjectRepository(PasswordHash) private readonly passwordRepository: Repository<PasswordHash>,
    @InjectRepository(Session) private readonly sessionRepository: Repository<Session>,
  ) {}

  @Get("me")
  @Header("Cache-Control", "no-store")
  @ApiOperation({ summary: "Read the verified operator profile and current organization roles" })
  @ApiResponse({ status: 200, description: "Profile from trusted app records and membership" })
  @ApiResponse({ status: 401, description: "Missing session, user or current membership" })
  @ApiResponse({ status: 503, description: "Identity dependency unavailable" })
  async me(@Req() req: Request) {
    const context = req.operatorContext;
    if (!context || req.user?.subjectId !== context.userId) {
      throw new UnauthorizedException("Missing verified operator context");
    }
    let user;
    try {
      user = await this.userRepository.findOne({
        where: { id: context.userId },
        select: { id: true, email: true, name: true },
      });
    } catch {
      throw new ServiceUnavailableException("Identity unavailable");
    }
    if (!user) throw new UnauthorizedException("User not found");
    return {
      id: user.id,
      email: user.email,
      name: user.name,
      organizationId: context.organizationId,
      roles: context.roles,
    };
  }

  @Public()
  @Post("sign-in")
  @HttpCode(200)
  @UsePipes(new ZodValidationPipe(signInSchema))
  @ApiOperation({ summary: "Sign in with email and password" })
  @ApiBody({
    schema: {
      type: "object",
      properties: { email: { type: "string" }, password: { type: "string" } },
    },
  })
  @ApiResponse({ status: 200, description: "Successfully signed in" })
  @ApiResponse({ status: 401, description: "Invalid credentials" })
  async signIn(@Body() body: SignInBody, @Res({ passthrough: true }) res: Response) {
    const { email, password } = body;

    // Cleanup expired sessions
    await this.sessionRepository.delete({ expiresAt: LessThan(new Date()) });

    const user = await this.userRepository.findOne({ where: { email } });
    const storedHash = user
      ? await this.passwordRepository.findOne({ where: { userId: user.id } })
      : null;

    // Use dummy hash if user or password hash not found to prevent timing attacks
    const isValid = await verifyPassword(password, storedHash ? storedHash.hash : DUMMY_HASH);

    if (!user || !isValid) {
      throw new UnauthorizedException("Invalid credentials");
    }

    const sessionId = randomUUID();
    const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000); // 24 hours

    await this.sessionRepository.save({
      id: hashSessionId(sessionId),
      userId: user.id,
      expiresAt,
    });

    res.cookie("session", sessionId, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      expires: expiresAt,
    });

    return { message: "Signed in successfully" };
  }

  @Public()
  @Post("sign-out")
  @HttpCode(200)
  @ApiOperation({ summary: "Sign out" })
  async signOut(@Req() req: Request, @Res({ passthrough: true }) res: Response) {
    const sessionCookie = readSessionCookie(req);
    if (sessionCookie) {
      await this.sessionRepository.delete({ id: hashSessionId(sessionCookie) });
    }

    res.clearCookie("session", {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
    });
    return { message: "Signed out successfully" };
  }
}
