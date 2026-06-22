const globals = require('globals');

module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: { jsx: true },
  },
  plugins: ['@typescript-eslint', 'jsx-a11y', 'react'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react/recommended',
    'plugin:jsx-a11y/recommended',
  ],
  rules: {
    // The project uses the new JSX transform (`jsx: "react-jsx"` in
    // tsconfig.json) so `import React` is not required. Disable the legacy
    // scope rules.
    'react/react-in-jsx-scope': 'off',
    'react/jsx-uses-react': 'off',
    'react/prop-types': 'off',
    'jsx-a11y/no-autofocus': 'warn',
  },
  settings: { react: { version: 'detect' } },
  globals: {
    ...globals.browser,
    ...globals.node,
    // Vitest globals (test, it, expect, describe, vi, beforeEach, afterEach).
    ...globals.vitest,
  },
};
