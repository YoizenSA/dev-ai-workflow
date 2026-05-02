# Biome Lint Rules

Guide for Biome linting rules and configuration in projects.

## Overview

Biome is used as a replacement for ESLint in projects. It provides:
- Fast linting
- Code formatting
- Integrated tooling
- TypeScript support

## Configuration File

Biome uses `biome.json` configuration file in each project.

Minimal baseline for new projects:

```json
{
  "$schema": "https://biomejs.dev/schemas/2.3.2/schema.json",
  "files": {
    "ignoreUnknown": true,
    "includes": [
      "**",
      "!!**/node_modules",
      "!!**/dist",
      "!!**/build",
      "!!**/coverage",
      "!!**/.next",
      "!!**/.nuxt",
      "!!**/.svelte-kit",
      "!!**/.turbo",
      "!!**/.vercel",
      "!!**/.cache",
      "!!**/__generated__",
      "!!**/*.generated.*",
      "!!**/*.gen.*",
      "!!**/generated",
      "!!**/codegen"
    ]
  },
  "formatter": {
    "enabled": true,
    "formatWithErrors": true,
    "indentStyle": "space",
    "indentWidth": 2,
    "lineEnding": "lf",
    "lineWidth": 80,
    "bracketSpacing": true
  },
  "assist": {
    "actions": {
      "source": {
        "organizeImports": "on",
        "useSortedAttributes": "on",
        "noDuplicateClasses": "on",
        "useSortedInterfaceMembers": "on",
        "useSortedProperties": "on"
      }
    }
  },
  "linter": {
    "enabled": true,
    "rules": {
      "correctness": {
        "noUnusedImports": {
          "fix": "safe",
          "level": "error"
        },
        "noUnusedVariables": "error",
        "noUnusedFunctionParameters": "error",
        "noUndeclaredVariables": "error",
        "useParseIntRadix": "warn",
        "useValidTypeof": "error",
        "noUnreachable": "error"
      },
      "style": {
        "useBlockStatements": {
          "fix": "safe",
          "level": "error"
        },
        "useConst": "error",
        "useImportType": "warn",
        "noNonNullAssertion": "error",
        "useTemplate": "warn"
      },
      "security": {
        "noGlobalEval": "error"
      },
      "suspicious": {
        "noExplicitAny": "error",
        "noImplicitAnyLet": "error",
        "noDoubleEquals": "warn",
        "noGlobalIsNan": "error",
        "noPrototypeBuiltins": "error"
      },
      "complexity": {
        "useOptionalChain": "error",
        "useLiteralKeys": "warn",
        "noForEach": "warn"
      },
      "nursery": {
        "useSortedClasses": {
          "fix": "safe",
          "level": "error",
          "options": {
            "attributes": ["className"],
            "functions": ["clsx", "cva", "tw", "twMerge", "cn", "twJoin", "tv"]
          }
        }
      }
    }
  },
  "javascript": {
    "formatter": {
      "arrowParentheses": "always",
      "semicolons": "always",
      "trailingCommas": "es5"
    }
  },
  "organizeImports": {
    "enabled": true
  },
  "vcs": {
    "enabled": true,
    "clientKind": "git",
    "useIgnoreFile": true,
    "defaultBranch": "main"
  }
}
```

## Rule Categories (installer baseline)

### Correctness Rules

**Enforce code correctness and prevent bugs.**

| Rule | Level | Description | Example |
|------|-------|-------------|---------|
| `noUnusedImports` | error (safe fix) | Disallow unused imports | `import { foo, bar } from 'x'` when `bar` is unused |
| `noUnusedVariables` | error | Disallow unused variables | `const x = 1;` (x not used) |
| `noUnusedFunctionParameters` | error | Disallow unused function parameters | `function f(x) { return 1; }` |
| `noUndeclaredVariables` | error | Disallow unknown globals/variables | `console.log(notDeclared)` |
| `useParseIntRadix` | warn | Require radix in parseInt | `parseInt('10')` → `parseInt('10', 10)` |
| `useValidTypeof` | error | Require valid `typeof` comparisons | `typeof x === 'strnig'` |
| `noUnreachable` | error | Disallow unreachable code | `return; console.log('never')` |

### Style Rules

**Enforce consistent code style.**

| Rule | Level | Description | Example |
|------|-------|-------------|---------|
| `useBlockStatements` | error (safe fix) | Require braces for control blocks | `if (ok) doWork();` |
| `useConst` | error | Use const when possible | `let x = 1;` (never reassigned) |
| `useImportType` | warn | Prefer type-only imports when applicable | `import { type Foo } from 'bar';` |
| `noNonNullAssertion` | error | Disallow non-null assertion | `x!` |
| `useTemplate` | warn | Prefer template literals over concatenation | `a + b` → `${a}${b}` |

### Suspicious Rules

**Detect potentially problematic code.**

| Rule | Level | Description | Example |
|------|-------|-------------|---------|
| `noExplicitAny` | error | Disallow explicit `any` | `const x: any = ...` |
| `noImplicitAnyLet` | error | Disallow implicit any in `let` | `let x;` |
| `noDoubleEquals` | warn | Disallow `==`, use `===` | `x == y` |
| `noGlobalIsNan` | error | Disallow global `isNaN` | `isNaN(x)` → `Number.isNaN(x)` |
| `noPrototypeBuiltins` | error | Disallow direct `hasOwnProperty` calls on unknown objects | `obj.hasOwnProperty('k')` |

### Complexity Rules

**Control code complexity.**

| Rule | Level | Description | Example |
|------|-------|-------------|---------|
| `useOptionalChain` | error | Prefer optional chaining | `obj && obj.prop` → `obj?.prop` |
| `useLiteralKeys` | warn | Prefer literal keys where possible | `obj['key']` → `obj.key` |
| `noForEach` | warn | Prefer loops/transform methods over forEach in many cases | `arr.forEach(...)` |

### Security Rules

| Rule | Level | Description | Example |
|------|-------|-------------|---------|
| `noGlobalEval` | error | Disallow global eval | `eval('code')` |

### Nursery Rules

| Rule | Level | Description |
|------|-------|-------------|
| `useSortedClasses` | error (safe fix) | Sort utility classes in `className` strings/functions |

## Rule Levels

| Level | Behavior | When to Fix |
|-------|----------|-------------|
| `error` | Fails lint/build | Must fix immediately |
| `warn` | Shows warning | Should fix when convenient |
| `off` | Disabled | Ignore the rule |

## Common Issues and Fixes

### Unused Variables/Imports

**Issue**: Biome reports unused imports/variables

**Fix**:
```typescript
// ❌ Error: Unused import
import { foo, bar } from './module';

function test() {
  return foo();
}

// ✅ Fix: Remove unused import
import { foo } from './module';

function test() {
  return foo();
}
```

### Use Const

**Issue**: Biome suggests using const instead of let

**Fix**:
```typescript
// ❌ Error: Use const
let x = 10;
console.log(x);

// ✅ Fix: Use const
const x = 10;
console.log(x);
```

### Double Equals

**Issue**: Biome warns about == instead of ===

**Fix**:
```typescript
// ❌ Warning: Use ===
if (x == 5) { ... }

// ✅ Fix: Use ===
if (x === 5) { ... }
```

### Optional Chain

**Issue**: Biome suggests optional chaining

**Fix**:
```typescript
// ❌ Error: Use optional chain
if (user && user.profile && user.profile.name) {
  console.log(user.profile.name);
}

// ✅ Fix: Use optional chain
if (user?.profile?.name) {
  console.log(user.profile.name);
}
```

## Customizing Rules

To customize rules for a project, edit `biome.json`:

```json
{
  "linter": {
    "rules": {
      "suspicious": {
        "noExplicitAny": "warn" // Relax from installer baseline (error) during migration
      },
      "style": {
        "useImportType": "error" // Change from "warn" to "error"
      }
    }
  }
}
```

## Finding Biome Configurations

```bash
# View WebApi biome config
cat Yoizen.this project.WebApi/biome.json

# View WebExecutor biome config
cat Yoizen.this project.WebExecutor/biome.json

# View all biome configs
find . -name "biome.json" -type f

# Compare configs across projects
diff Yoizen.this project.WebApi/biome.json Yoizen.this project.WebExecutor/biome.json
```

## Related Skills

- **`biome`** - Biome commands and format configuration
- **`webapi`** - NestJS backend linting
- **`testing`** - Test file linting
