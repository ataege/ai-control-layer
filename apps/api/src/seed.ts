import { NestFactory } from "@nestjs/core";
import { AppModule } from "./app.module.js";
import { AppConfigService } from "./config/app-config.service.js";
import { getRepositoryToken } from "@nestjs/typeorm";
import { User } from "./identity/entities/user.entity.js";
import { Organization } from "./identity/entities/organization.entity.js";
import { Membership } from "./identity/entities/membership.entity.js";
import { PasswordHash } from "./identity/entities/password-hash.entity.js";
import { hashPassword } from "./auth/password.util.js";
import { createApplicationLogger } from "./common/logger.js";
import { DataSource } from "typeorm";

async function bootstrap() {
  const logger = createApplicationLogger(process.env.LOG_LEVEL);
  const app = await NestFactory.createApplicationContext(AppModule, { logger, abortOnError: false });
  const config = app.get(AppConfigService);
  
  const userRepository = app.get(getRepositoryToken(User));
  const orgRepository = app.get(getRepositoryToken(Organization));
  const membershipRepository = app.get(getRepositoryToken(Membership));
  const passwordRepository = app.get(getRepositoryToken(PasswordHash));
  
  await app.init();
  const dataSource = app.get(DataSource);
  while (!dataSource.isInitialized) {
    await new Promise(resolve => setTimeout(resolve, 500));
  }

  const email = "demo@example.com";
  const orgName = "Acme Corp";
  const password = config.demoOperatorPassword;

  if (!password) {
    logger.error("DEMO_OPERATOR_PASSWORD is empty in environment", "Seed");
    process.exit(1);
  }

  let org = await orgRepository.findOne({ where: { name: orgName } });
  if (!org) {
    org = await orgRepository.save({ name: orgName });
    logger.log(`Created organization ${orgName}`, "Seed");
  }

  let user = await userRepository.findOne({ where: { email } });
  if (!user) {
    user = await userRepository.save({ email, name: "Demo Operator" });
    logger.log(`Created user ${email}`, "Seed");
    
    await passwordRepository.save({
      userId: user.id,
      hash: await hashPassword(password),
    });

    await membershipRepository.save({
      userId: user.id,
      organizationId: org.id,
      roles: ["admin"],
    });
    
    logger.log(`Seed complete. Login with ${email} and the generated DEMO_OPERATOR_PASSWORD.`, "Seed");
  } else {
    logger.log("Demo user already exists.", "Seed");
  }
  
  await app.close();
}

bootstrap().catch((err) => {
  console.error("Seed failed:", err);
  process.exit(1);
});
