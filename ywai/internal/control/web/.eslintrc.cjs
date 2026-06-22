const globals = require('globals');

module.exports = {
  root: true,
  env: { browser: true, node: true, es2022: true },
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
    // Brief originally specified 'jsx-a11y/aria-label' but that rule does not
    // exist in eslint-plugin-jsx-a11y v6 (the closest valid rules are
    // 'aria-props' and 'aria-role'). Use 'aria-props' to catch missing
    // ARIA attributes on JSX elements.
    'jsx-a11y/aria-props': 'error',
    'jsx-a11y/aria-role': 'error',
    'jsx-a11y/no-autofocus': 'warn',
    'jsx-a11y/tabindex-no-positive': 'error',
    'react/prop-types': 'off',
    // The project uses the new JSX transform (`jsx: "react-jsx"` in
    // tsconfig.json) so `import React` is not required. Disable the legacy
    // scope rules.
    'react/react-in-jsx-scope': 'off',
    'react/jsx-uses-react': 'off',
  },
  settings: { react: { version: 'detect' } },
  globals: {
    ...globals.browser,
    ...globals.node,
    // Vitest globals (test, it, expect, describe, vi, beforeEach, afterEach).
    ...globals.vitest,
  },
};
