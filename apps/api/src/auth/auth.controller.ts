import {
  Controller,
  Post,
  Body,
  Res,
  Req,
  HttpCode,
  UnauthorizedException,
  UsePipes,
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

const signInSchema = z.object({
  email: z.string().email(),
  password: z.string().max(256),
});

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
  async signIn(@Body() body: any, @Res({ passthrough: true }) res: Response) {
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
    const sessionCookie = req.cookies?.session;
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
