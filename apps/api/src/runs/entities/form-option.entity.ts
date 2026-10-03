import { Column, Entity, PrimaryGeneratedColumn } from "typeorm";
import { TaskFormOptions } from "@workspace/contracts";

@Entity({ name: "form_options", schema: "app" })
export class FormOption {
  @PrimaryGeneratedColumn("uuid")
  id: string;

  @Column({ name: "organization_id", type: "uuid" })
  organizationId: string;

  @Column({ type: "jsonb" })
  options: TaskFormOptions;
}
