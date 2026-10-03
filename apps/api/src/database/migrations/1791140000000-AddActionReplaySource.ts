import type { MigrationInterface, QueryRunner } from "typeorm";

// W3 Go lane (action gate), GO-36, approved by the lead (timestamp assigned). The labelled replay
// (GO-05) marks a replayed proposal in the stored action itself: replay_source is
// labelled_replay:<fixture id> for a replay and NULL for a model proposal ("never present a
// scripted proposal as a model-generated action"). Every event of the action carries the same
// value in maskedSummary.replaySource (X-12); X-09 StoredAction.replaySource mirrors the column.
export class AddActionReplaySource1791140000000 implements MigrationInterface {
  name = "AddActionReplaySource1791140000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "runtime"."actions"
         ADD COLUMN "replay_source" text,
         ADD CONSTRAINT "actions_replay_source_label"
           CHECK ("replay_source" IS NULL
                  OR (char_length("replay_source") <= 256
                      AND "replay_source" ~ '^labelled_replay:[^[:cntrl:]]+$'))`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "runtime"."actions" DROP CONSTRAINT "actions_replay_source_label", DROP COLUMN "replay_source"`,
    );
  }
}
