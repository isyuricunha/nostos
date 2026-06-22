import js from '@eslint/js';
import tseslint from '@typescript-eslint/eslint-plugin';
import tsParser from '@typescript-eslint/parser';
import svelte from 'eslint-plugin-svelte';

const browserGlobals = {
  document: 'readonly',
  fetch: 'readonly',
  Response: 'readonly',
  window: 'readonly'
};

const testGlobals = {
  describe: 'readonly',
  expect: 'readonly',
  it: 'readonly',
  vi: 'readonly'
};

export default [
  js.configs.recommended,
  ...svelte.configs['flat/recommended'],
  {
    files: ['src/**/*.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        project: './tsconfig.json',
        tsconfigRootDir: import.meta.dirname
      },
      globals: browserGlobals
    },
    plugins: {
      '@typescript-eslint': tseslint
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'error'
    }
  },
  {
    files: ['src/**/*.svelte'],
    languageOptions: {
      parserOptions: {
        parser: tsParser,
        project: './tsconfig.json',
        extraFileExtensions: ['.svelte'],
        tsconfigRootDir: import.meta.dirname
      },
      globals: browserGlobals
    },
    plugins: {
      '@typescript-eslint': tseslint
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'error'
    }
  },
  {
    files: ['src/**/*.test.ts'],
    languageOptions: {
      globals: {
        ...browserGlobals,
        ...testGlobals
      }
    }
  },
  {
    files: ['vite.config.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        project: './tsconfig.node.json',
        tsconfigRootDir: import.meta.dirname
      }
    },
    plugins: {
      '@typescript-eslint': tseslint
    },
    rules: {
      ...tseslint.configs.recommended.rules
    }
  },
  {
    ignores: ['dist', 'node_modules']
  }
];
