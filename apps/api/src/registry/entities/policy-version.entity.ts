import {
  Column,
  CreateDateColumn,
  Entity,
  PrimaryGeneratedColumn,
  JoinColumn,
  ManyToOne,
} from "typeorm";
import { Organization } from "../../identity/entities/organization.entity.js";

@Entity({ name: "policy_versions", schema: "app" })
export class PolicyVersion {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column({ name: "organization_id", type: "uuid" })
  organizationId: string;

  // The foreign key is created by AddAppOrganizationForeignKeys; the entity declares it so the
  // schema diff stays empty.
  @ManyToOne(() => Organization, { onDelete: "CASCADE" })
  @JoinColumn({ name: "organization_id", foreignKeyConstraintName: "FK_policy_org" })
  organization: Organization;

  @Column({ type: "varchar" })
  name: string;

  @Column({ type: "jsonb" })
  rules: unknown;

  @Column({ type: "varchar", nullable: true })
  description: string;

  @CreateDateColumn({ name: "created_at" })
  createdAt: Date;
}
