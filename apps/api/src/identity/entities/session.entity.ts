import { Entity, PrimaryColumn, Column, CreateDateColumn, ManyToOne, JoinColumn } from "typeorm";
import { User } from "./user.entity.js";

@Entity({ schema: "app", name: "sessions" })
export class Session {
  @PrimaryColumn("text")
  id: string;

  @Column("uuid")
  userId: string;

  @Column({ type: "timestamptz" })
  expiresAt: Date;

  @ManyToOne(() => User, { onDelete: "CASCADE" })
  @JoinColumn({ name: "userId" })
  user: User;

  @CreateDateColumn({ type: "timestamptz" })
  createdAt: Date;
}
