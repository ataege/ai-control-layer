import { Module } from "@nestjs/common";
import { TypeOrmModule } from "@nestjs/typeorm";
import { PolicyVersion } from "./entities/policy-version.entity.js";
import { TaskTemplate } from "./entities/task-template.entity.js";
import { ToolDefinition } from "./entities/tool-definition.entity.js";

@Module({
  imports: [TypeOrmModule.forFeature([TaskTemplate, PolicyVersion, ToolDefinition])],
  exports: [TypeOrmModule],
})
export class RegistryModule {}
