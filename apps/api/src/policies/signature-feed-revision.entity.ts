import { Check, Column, CreateDateColumn, Entity, PrimaryGeneratedColumn, Unique } from "typeorm";
import type { CatalogImportSource } from "./control-catalog-revision.entity.js";

/**
 * One immutable, trusted import of the attack-signature feed (SH-46, imported by API-34). Only feeds
 * that passed issuer, integrity and grammar validation become rows. A database trigger rejects every
 * UPDATE, DELETE and TRUNCATE.
 */
@Entity({ name: "signature_feed_revisions", schema: "app" })
@Unique("signature_feed_revisions_issuer_revision", ["issuer", "revision"])
@Check("signature_feed_revisions_file_digest_format", `"file_digest" ~ '^[0-9a-f]{64}$'`)
@Check(
  "signature_feed_revisions_import_actor",
  `("import_source" = 'command' AND "imported_by" IS NULL) OR ("import_source" = 'reload' AND "imported_by" IS NOT NULL)`,
)
export class SignatureFeedRevision {
  @PrimaryGeneratedColumn("identity", { type: "bigint", generatedIdentity: "ALWAYS" })
  id!: string;

  /** The trusted publisher the feed was authenticated against. */
  @Column({ name: "issuer", type: "text" })
  issuer!: string;

  /** The feed's own revision label, which `signatures.revision` in policy.yaml refers to. */
  @Column({ name: "revision", type: "text" })
  revision!: string;

  @Column({ name: "source_file_name", type: "text" })
  sourceFileName!: string;

  @Column({ name: "source_text", type: "text" })
  sourceText!: string;

  /** Lowercase hex SHA-256 of the file's UTF-8 bytes; an audit field, not proof of the publisher. */
  @Column({ name: "file_digest", type: "text" })
  fileDigest!: string;

  @Column({ name: "content", type: "jsonb" })
  content!: Record<string, unknown>;

  @Column({ name: "import_source", type: "text" })
  importSource!: CatalogImportSource;

  @Column({ name: "imported_by", type: "text", nullable: true })
  importedBy!: string | null;

  @CreateDateColumn({ name: "created_at", type: "timestamptz" })
  createdAt!: Date;
}
