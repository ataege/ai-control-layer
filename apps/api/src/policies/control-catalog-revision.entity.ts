import { Check, Column, CreateDateColumn, Entity, PrimaryGeneratedColumn } from "typeorm";

/**
 * How a revision entered the catalog. `command` is the explicit import command, which has no
 * authenticated actor; `reload` is the authenticated reload, which always names its actor.
 */
export type CatalogImportSource = "command" | "reload";

/**
 * One immutable, schema-valid import of `policy.yaml` (config/README.md). Only files that passed
 * validation become rows; a rejected import is recorded on the pointer instead, so nothing invalid
 * can ever be activated. A database trigger rejects every UPDATE, DELETE and TRUNCATE.
 */
@Entity({ name: "control_catalog_revisions", schema: "app" })
@Check("control_catalog_revisions_file_digest_format", `"file_digest" ~ '^[0-9a-f]{64}$'`)
@Check(
  "control_catalog_revisions_import_actor",
  `("import_source" = 'command' AND "imported_by" IS NULL) OR ("import_source" = 'reload' AND "imported_by" IS NOT NULL)`,
)
export class ControlCatalogRevision {
  /** Sequential revision number; also the label shown to operators (revision 1, 2, ...). */
  @PrimaryGeneratedColumn("identity", { type: "bigint", generatedIdentity: "ALWAYS" })
  id!: string;

  /** The file's `schema_version`, kept as a column so readers can reject an unknown schema early. */
  @Column({ name: "schema_version", type: "integer" })
  schemaVersion!: number;

  @Column({ name: "source_file_name", type: "text" })
  sourceFileName!: string;

  /** The exact file text, so the digest can be recomputed and the revision audited. */
  @Column({ name: "source_text", type: "text" })
  sourceText!: string;

  /** Lowercase hex SHA-256 of the file's UTF-8 bytes. */
  @Column({ name: "file_digest", type: "text" })
  fileDigest!: string;

  /** The validated document as parsed; Go evaluates against this, not against the file. */
  @Column({ name: "content", type: "jsonb" })
  content!: Record<string, unknown>;

  @Column({ name: "import_source", type: "text" })
  importSource!: CatalogImportSource;

  /** Authenticated actor reference for a reload; becomes a foreign key once users exist (SH-15). */
  @Column({ name: "imported_by", type: "text", nullable: true })
  importedBy!: string | null;

  @CreateDateColumn({ name: "created_at", type: "timestamptz" })
  createdAt!: Date;
}
