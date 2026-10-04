import {
  Column,
  CreateDateColumn,
  Entity,
  PrimaryGeneratedColumn,
  JoinColumn,
  ManyToOne,
} from "typeorm";
import { Organization } from "../../identity/entities/organization.entity.js";

@Entity({ name: "tool_definitions", schema: "app" })
export class ToolDefinition {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column({ name: "organization_id", type: "uuid" })
  organizationId: string;

  // The foreign key is created by AddAppOrganizationForeignKeys; the entity declares it so the
  // schema diff stays empty.
  @ManyToOne(() => Organization, { onDelete: "CASCADE" })
  @JoinColumn({ name: "organization_id", foreignKeyConstraintName: "FK_tool_org" })
  organization: Organization;

  // e.g. read_invoice, read_vendor, create_report, queue_report
  @Column({ type: "varchar" })
  name: string;

  @Column({ type: "jsonb" })
  schema: unknown;

  @CreateDateColumn({ name: "created_at" })
  createdAt: Date;
}
