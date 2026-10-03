import { Controller, Post, Body, Res, HttpCode, UnauthorizedException } from "@nestjs/common";
import type { Response } from "express";
import { InjectRepository } from "@nestjs/typeorm";
import { Repository } from "typeorm";
import { randomUUID } from "node:crypto";
import { Public } from "./public.decorator.js";
import { User } from "../identity/entities/user.entity.js";
import { PasswordHash } from "../identity/entities/password-hash.entity.js";
import { Session } from "../identity/entities/session.entity.js";
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from "@nestjs/swagger";

@ApiTags("Auth")
@Controller("auth")
export class AuthController {
  constructor(
    @InjectRepository(User) private readonly userRepository: Repository<User>,
    @InjectRepository(PasswordHash) private readonly passwordRepository: Repository<PasswordHash>,
    @InjectRepository(Session) private readonly sessionRepository: Repository<Session>
  ) {}

  @Public()
  @Post("sign-in")
  @HttpCode(200)
  @ApiOperation({ summary: "Sign in with email and password" })
  @ApiBody({ schema: { type: "object", properties: { email: { type: "string" }, password: { type: "string" } } } })
  @ApiResponse({ status: 200, description: "Successfully signed in" })
  @ApiResponse({ status: 401, description: "Invalid credentials" })
  async signIn(@Body() body: any, @Res({ passthrough: true }) res: Response) {
    const { email, password } = body;
    const user = await this.userRepository.findOne({ where: { email } });
    if (!user) throw new UnauthorizedException("Invalid credentials");

    const passHash = await this.passwordRepository.findOne({ where: { userId: user.id } });
    if (!passHash || passHash.hash !== password) {
      throw new UnauthorizedException("Invalid credentials");
    }

    const sessionId = randomUUID();
    const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000); // 24 hours

    await this.sessionRepository.save({
      id: sessionId,
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

  @Post("sign-out")
  @HttpCode(200)
  @ApiOperation({ summary: "Sign out" })
  async signOut(@Res({ passthrough: true }) res: Response) {
    res.clearCookie("session");
    return { message: "Signed out successfully" };
  }
}
