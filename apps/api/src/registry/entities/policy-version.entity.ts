import { Column, CreateDateColumn, Entity, PrimaryGeneratedColumn } from "typeorm";

@Entity({ name: "policy_versions", schema: "app" })
export class PolicyVersion {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column({ name: "organization_id", type: "uuid" })
  organizationId: string;

  @Column({ type: "varchar" })
  name: string;

  @Column({ type: "jsonb" })
  rules: unknown;

  @Column({ type: "varchar", nullable: true })
  description: string;

  @CreateDateColumn({ name: "created_at" })
  createdAt: Date;
}
