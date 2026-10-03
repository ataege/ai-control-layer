import { createConfig } from "@workspace/config/eslint";

export default createConfig({ tsconfigRootDir: import.meta.dirname, runtime: "browser" });
