import { ConflictException, Injectable, ServiceUnavailableException } from "@nestjs/common";
import { DataSource } from "typeorm";
import { importPolicyFile, type PolicyImportOutcome } from "./policy-catalog-importer.js";
import { ControlCatalogPointer } from "./control-catalog-pointer.entity.js";
import { readPolicyFiles, REPOSITORY_POLICY_PATH } from "./read-policy-files.js";
import { SignatureFeedRevision } from "./signature-feed-revision.entity.js";

@Injectable()
export class PoliciesService {
  constructor(private readonly database: DataSource) {}

  async reload(
    userId: string,
  ): Promise<{ outcome: PolicyImportOutcome; feedRevision: string | null }> {
    if (!this.database.isInitialized) throw new ServiceUnavailableException("Database unavailable");
    try {
      const files = await readPolicyFiles(REPOSITORY_POLICY_PATH);
      return await this.database.transaction(async (manager) => {
        const outcome = await importPolicyFile(manager, {
          ...files,
          provenance: { importSource: "reload", importedBy: userId },
        });
        let feedRevision: string | null = null;
        if (outcome.accepted && outcome.feedRevisionId !== null) {
          const feed = await manager.findOneByOrFail(SignatureFeedRevision, {
            id: outcome.feedRevisionId,
          });
          feedRevision = feed.revision;
        }
        return { outcome, feedRevision };
      });
    } catch (error) {
      if ((error as { code?: unknown }).code === "55P03") {
        throw new ConflictException({
          code: "revision_pending",
          message: "Another catalog update is pending",
        });
      }
      throw new ServiceUnavailableException("Policy reload unavailable");
    }
  }

  async pointer(): Promise<ControlCatalogPointer> {
    if (!this.database.isInitialized) throw new ServiceUnavailableException("Database unavailable");
    try {
      const pointer = await this.database.manager.findOneBy(ControlCatalogPointer, { id: 1 });
      if (!pointer) throw new Error("Catalog pointer missing");
      return pointer;
    } catch {
      throw new ServiceUnavailableException("Policy status unavailable");
    }
  }
}
