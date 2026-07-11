import js from "@eslint/js";
import { defineConfig, globalIgnores } from "eslint/config";
import jsxA11y from "eslint-plugin-jsx-a11y";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";
import simpleImportSort from "eslint-plugin-simple-import-sort";
import unusedImports from "eslint-plugin-unused-imports";
import globals from "globals";
import tseslint from "typescript-eslint";

import preferAppAlias from "./eslint-rules/prefer-app-alias.js";

const sourceFiles = ["**/*.{js,jsx,ts,tsx}"];
const browserGlobals = {
  ...globals.browser,
  ...globals.es2022,
};
const nodeTestGlobals = {
  ...Object.fromEntries(Object.keys(globals.browser).map((name) => [name, "off"])),
  ...globals.es2022,
  ...globals.node,
};

export default defineConfig([
  globalIgnores([
    ".react-router/**",
    "build/**",
    "coverage/**",
    "node_modules/**",
    "playwright-report/**",
    "test-results/**",
  ]),
  {
    files: sourceFiles,
    languageOptions: {
      ecmaVersion: "latest",
      globals: browserGlobals,
      parserOptions: {
        ecmaFeatures: {
          jsx: true,
        },
      },
      sourceType: "module",
    },
    settings: {
      react: {
        version: "detect",
      },
    },
    plugins: {
      sandrone: {
        rules: {
          "prefer-app-alias": preferAppAlias,
        },
      },
      "simple-import-sort": simpleImportSort,
      "unused-imports": unusedImports,
    },
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: sourceFiles,
    ...react.configs.flat.recommended,
  },
  {
    files: sourceFiles,
    ...react.configs.flat["jsx-runtime"],
  },
  {
    files: sourceFiles,
    ...reactHooks.configs.flat.recommended,
  },
  {
    files: sourceFiles,
    ...jsxA11y.flatConfigs.recommended,
    languageOptions: {
      ...jsxA11y.flatConfigs.recommended.languageOptions,
      globals: browserGlobals,
    },
  },
  {
    files: sourceFiles,
    rules: {
      "@typescript-eslint/no-unused-vars": "off",
      "comma-spacing": ["error", { "after": true, "before": false }],
      "no-unused-vars": "off",
      "sandrone/prefer-app-alias": "error",
      "simple-import-sort/exports": "error",
      "simple-import-sort/imports": [
        "error",
        {
          groups: [
            ["^\\u0000"],
            ["^node:"],
            ["^react$", "^react", "^react-router", "^@?\\w"],
            ["^~/"],
            ["^\\."],
          ],
        },
      ],
      "unused-imports/no-unused-imports": "error",
      "unused-imports/no-unused-vars": [
        "error",
        {
          args: "after-used",
          argsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
        },
      ],
      "react/prop-types": "off",
      "react-hooks/set-state-in-effect": "off",
    },
  },
  {
    files: ["app/routes/**/*.{ts,tsx}"],
    ignores: ["app/routes/**/*.test.{ts,tsx}", "app/routes/**/*.dom.test.ts"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@mui/**"],
              message: "Compose presentation in core, features, or shared modules instead of public routes.",
            },
          ],
        },
      ],
    },
  },
  {
    files: [
      "app/**/*.test.{ts,tsx}",
      "app/test/**/*.{ts,tsx}",
      "playwright.config.ts",
      "react-router.config.ts",
      "vite.config.ts",
    ],
    languageOptions: {
      globals: {
        ...browserGlobals,
        ...globals.node,
      },
    },
  },
  {
    files: ["app/**/*.test.ts"],
    ignores: ["app/**/*.dom.test.ts"],
    languageOptions: {
      globals: nodeTestGlobals,
    },
    rules: {
      "no-undef": "error",
      "no-restricted-globals": [
        "error",
        "localStorage",
        "sessionStorage",
        "document",
        "window",
        "navigator",
      ],
    },
  },
]);
