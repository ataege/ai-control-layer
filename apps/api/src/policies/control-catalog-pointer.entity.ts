import {
  Check,
  Column,
  Entity,
  JoinColumn,
  ManyToOne,
  PrimaryColumn,
  UpdateDateColumn,
} from "typeorm";
import { ControlCatalogRevision } from "./control-catalog-revision.entity.js";
import { SignatureFeedRevision } from "./signature-feed-revision.entity.js";

/**
 * The single active-version pointer of the control catalog. It holds the three states of a reload:
 * the requested revision (imported, waiting for Go), the validated revision (acknowledged by Go) and
 * the active revision (what Go evaluates against). How a revision moves between them is the open item
 * `catalog activation protocol`. A rejected reload leaves the active revision in place and records
 * the error here, so the last-known-good revision stays visible together with the failure.
 */
@Entity({ name: "control_catalog_pointer", schema: "app" })
@Check("control_catalog_pointer_single_row", `"id" = 1`)
@Check(
  "control_catalog_pointer_error_pair",
  `("last_error" IS NULL AND "last_error_at" IS NULL) OR ("last_error" IS NOT NULL AND "last_error_at" IS NOT NULL)`,
)
export class ControlCatalogPointer {
  /** Always 1: there is exactly one catalog. */
  @PrimaryColumn({ name: "id", type: "smallint" })
  id!: number;

  @ManyToOne(() => ControlCatalogRevision, { nullable: true, onDelete: "RESTRICT" })
  @JoinColumn({ name: "requested_revision_id" })
  requestedRevision!: ControlCatalogRevision | null;

  @Column({ name: "requested_revision_id", type: "bigint", nullable: true })
  requestedRevisionId!: string | null;

  @ManyToOne(() => ControlCatalogRevision, { nullable: true, onDelete: "RESTRICT" })
  @JoinColumn({ name: "validated_revision_id" })
  validatedRevision!: ControlCatalogRevision | null;

  @Column({ name: "validated_revision_id", type: "bigint", nullable: true })
  validatedRevisionId!: string | null;

  @ManyToOne(() => ControlCatalogRevision, { nullable: true, onDelete: "RESTRICT" })
  @JoinColumn({ name: "active_revision_id" })
  activeRevision!: ControlCatalogRevision | null;

  /** Null until the first accepted activation; with no active revision Go dispatches nothing. */
  @Column({ name: "active_revision_id", type: "bigint", nullable: true })
  activeRevisionId!: string | null;

  @ManyToOne(() => SignatureFeedRevision, { nullable: true, onDelete: "RESTRICT" })
  @JoinColumn({ name: "active_feed_revision_id" })
  activeFeedRevision!: SignatureFeedRevision | null;

  /** The feed revision bound to the active catalog revision, resolved at activation. */
  @Column({ name: "active_feed_revision_id", type: "bigint", nullable: true })
  activeFeedRevisionId!: string | null;

  /** The latest rejected import or reload: a stable reason code and safe details, never secrets. */
  @Column({ name: "last_error", type: "jsonb", nullable: true })
  lastError!: Record<string, unknown> | null;

  @Column({ name: "last_error_at", type: "timestamptz", nullable: true })
  lastErrorAt!: Date | null;

  @UpdateDateColumn({ name: "updated_at", type: "timestamptz" })
  updatedAt!: Date;
}
