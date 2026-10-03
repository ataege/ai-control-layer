import { Entity, PrimaryGeneratedColumn, Column, CreateDateColumn } from "typeorm";

@Entity({ schema: "app", name: "organizations" })
export class Organization {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column({ unique: true })
  name: string;

  @CreateDateColumn({ type: "timestamptz" })
  createdAt: Date;
}
