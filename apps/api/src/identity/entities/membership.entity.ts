import {
  Entity,
  PrimaryGeneratedColumn,
  Column,
  CreateDateColumn,
  ManyToOne,
  JoinColumn,
  Unique,
} from "typeorm";
import { User } from "./user.entity.js";
import { Organization } from "./organization.entity.js";

@Entity({ schema: "app", name: "memberships" })
@Unique(["userId", "organizationId"])
export class Membership {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column()
  userId: string;

  @Column()
  organizationId: string;

  @Column("text", { array: true, default: "{}" })
  roles: string[];

  @ManyToOne(() => User)
  @JoinColumn({ name: "userId" })
  user: User;

  @ManyToOne(() => Organization)
  @JoinColumn({ name: "organizationId" })
  organization: Organization;

  @CreateDateColumn({ type: "timestamptz" })
  createdAt: Date;
}
