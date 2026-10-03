import { createConfig } from "@workspace/config/eslint";

export default createConfig({
  tsconfigRootDir: import.meta.dirname,
  typeChecked: true,
  extraConfigs: [
    {
      // The TypeORM CLI template leaves `queryRunner` unused until the migration is filled in.
      files: ["src/database/migrations/**/*.ts"],
      rules: { "@typescript-eslint/no-unused-vars": "off" },
    },
  ],
});
