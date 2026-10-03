import { Column, CreateDateColumn, Entity, PrimaryGeneratedColumn } from "typeorm";

@Entity({ name: "tool_definitions", schema: "app" })
export class ToolDefinition {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column({ name: "organization_id", type: "uuid" })
  organizationId: string;

  // e.g. read_invoice, read_vendor, create_report, queue_report
  @Column({ type: "varchar" })
  name: string;

  @Column({ type: "jsonb" })
  schema: unknown;

  @CreateDateColumn({ name: "created_at" })
  createdAt: Date;
}
