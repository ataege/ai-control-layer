import { Module } from "@nestjs/common";
import { TypeOrmModule } from "@nestjs/typeorm";
import { User } from "./entities/user.entity.js";
import { Organization } from "./entities/organization.entity.js";
import { Membership } from "./entities/membership.entity.js";
import { PasswordHash } from "./entities/password-hash.entity.js";
import { Session } from "./entities/session.entity.js";

@Module({
  imports: [TypeOrmModule.forFeature([User, Organization, Membership, PasswordHash, Session])],
  exports: [TypeOrmModule],
})
export class IdentityModule {}
