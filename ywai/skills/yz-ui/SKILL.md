---
name: yz-ui
description: >
  Yoizen UI design system guidelines and resources.
  Trigger: When working on Yoizen UI components, styling, colors, typography, or icons.
license: Apache-2.0
metadata:
  author: Yoizen
  version: "1.0"
  scope: [root]
  auto_invoke:
    - "ui design"
    - "yoizen"
    - "styling"
    - "components"
---

## When to Use

Use this skill when:
- Creating new UI components for Yoizen
- Implementing consistent styling across the application
- Choosing colors, fonts, or spacing
- Working with icons and brand assets
- Ensuring design system compliance

## Critical Patterns

### Color Palette

**Primary Colors:**
| Color | Hex | Usage |
|-------|-----|-------|
| Primary | `#1A66FF` | Buttons, links, primary actions |
| Secondary | `#4A3ABF` | Purple accents, gradients |
| Accent | `#FD6421` | Orange highlights, warnings |
| Yellow | `#FDBD27` | Auxiliary highlights |

**Background Colors:**
| Color | Hex | Usage |
|-------|-----|-------|
| Background | `#1a1a1a` | Main page background |
| Surface | `#2d2d2d` | Cards, panels, surfaces |
| Card | `#3a3a3a` | Individual card elements |
| Sidebar | `#050505` | Navigation sidebar |

**Text Colors:**
| Color | Hex | Usage |
|-------|-----|-------|
| Primary Text | `#ffffff` | Headlines, primary content |
| Secondary Text | `#b3b3b3` | Descriptions, secondary info |
| Muted | `#808080` | Placeholders, disabled text |

**Utility Colors:**
| Color | Hex | Usage |
|-------|-----|-------|
| Border | `#404040` | Dividers, borders |
| Hover Accent | `#2563eb` | Button/link hover states |

### Typography

**Font Families:**
- **Primary**: `Barlow, sans-serif` - Main body text
- **Sans**: `Plus Jakarta Sans, Inter, system-ui, sans-serif` - UI elements
- **Mono**: `JetBrains Mono, ui-monospace, monospace` - Code, technical text

**Font Sizes (Standard):**
- Headlines: Use Tailwind's `text-2xl` to `text-4xl`
- Body: `text-base` (16px)
- Small: `text-sm` (14px)
- Extra Small: `text-xs` (12px)

### Spacing

**Base Unit**: 4px (0.25rem in Tailwind)

**Common Spacing Scale:**
| Token | Value | Usage |
|-------|-------|-------|
| xs | 4px | Tight gaps |
| sm | 8px | Small padding |
| md | 16px | Standard padding |
| lg | 24px | Large gaps |
| xl | 32px | Section spacing |

### Border Radius

- **Small**: `0.375rem` (6px) - Buttons, inputs
- **Medium**: `0.5rem` (8px) - Cards, panels
- **Large**: `0.75rem` (12px) - Modals, large cards
- **Full**: `9999px` - Pills, badges

### Gradients

**Brand Gradient:**
```css
background: linear-gradient(90deg, #1A66FF 0%, #4A3ABF 60%, #FD6421 100%);
```

**Tailwind Class:**
```html
<div class="bg-brand-gradient">
```

### Shadows

**Standard Shadow:**
```css
box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.3);
```

**Card Shadow (Tailwind):**
```html
<div class="shadow-lg">
```

## Code Examples

### Basic Card Component

```tsx
<div className="bg-card border border-subtle rounded-lg p-4 shadow-lg">
  <h3 className="text-lg font-semibold text-primary">Card Title</h3>
  <p className="text-secondary mt-2">Card content description</p>
</div>
```

### Button Variants

```tsx
// Primary Button
<button className="bg-primary hover:bg-blue-600 text-white px-4 py-2 rounded-md transition-all">
  Primary Action
</button>

// Secondary Button
<button className="bg-surface border border-subtle hover:bg-card text-white px-4 py-2 rounded-md transition-all">
  Secondary Action
</button>

// Accent Button
<button className="bg-accent hover:bg-orange-600 text-white px-4 py-2 rounded-md transition-all">
  Warning/Highlight
</button>
```

### Input Field

```tsx
<input
  className="w-full bg-surface border border-subtle rounded-md px-3 py-2 text-primary
             focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary
             placeholder:text-muted"
  placeholder="Enter text..."
/>
```

### Alert Messages

```tsx
// Info Alert
<div className="bg-blue-500/10 border border-primary rounded-md p-3 text-primary">
  Information message
</div>

// Success Alert
<div className="bg-green-500/10 border border-green-500 rounded-md p-3 text-primary">
  Success message
</div>

// Warning Alert
<div className="bg-orange-500/10 border border-accent rounded-md p-3 text-primary">
  Warning message
</div>

// Error Alert
<div className="bg-red-500/10 border border-red-500 rounded-md p-3 text-primary">
  Error message
</div>
```

### Layout Patterns

```tsx
// Sidebar Layout
<div className="flex h-screen">
  <aside className="w-64 bg-sidebar border-r border-subtle">
    {/* Navigation */}
  </aside>
  <main className="flex-1 bg-background overflow-auto p-6">
    {/* Content */}
  </main>
</div>
```

### Using CSS Variables

```css
.my-custom-component {
  background-color: var(--surface-bg);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.my-custom-component:hover {
  border-color: var(--brand-blue);
}
```

## Brand Assets

### Available Icons

Located in `Services/yoizen-ui/public/`:

| File | Usage |
|------|-------|
| `logo.svg` | Main logo |
| `logo-sec-slogan.svg` | Logo with slogan |
| `logo-negativo.svg` | Negative/inverted logo |
| `logo-negative.svg` | Alternative negative |
| `logo-footer.svg` | Footer optimized |
| `logo-dorso-maneas.svg` | Special variant |
| `icon.svg` | Favicon/icon |

### Usage Example

```tsx
<img src="/logo.svg" alt="Yoizen Logo" className="h-8 w-auto" />
```

## Tailwind Configuration

The Yoizen UI extends Tailwind with custom theme values:

```javascript
// tailwind.config.js
export default {
  theme: {
    extend: {
      fontFamily: {
        sans: ["Plus Jakarta Sans", "Inter", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
        barlow: ['Barlow', 'sans-serif'],
      },
      colors: {
        primary: '#1A66FF',
        secondary: '#4A3ABF',
        accent: '#FD6421',
        yellow: '#FDBD27',
        background: '#1a1a1a',
        surface: '#2d2d2d',
        card: '#3a3a3a',
        muted: '#808080',
        subtle: '#404040',
      },
    },
  },
};
```

## Commands

```bash
# Install required fonts
npm install @fontsource/barlow @fontsource/plus-jakarta-sans @fontsource/jetbrains-mono

# Import fonts in main entry
import '@fontsource/barlow/400.css';
import '@fontsource/barlow/600.css';
import '@fontsource/plus-jakarta-sans/400.css';
import '@fontsource/plus-jakarta-sans/600.css';
import '@fontsource/jetbrains-mono/400.css';
```

## Best Practices

### DO:
- Use CSS variables for consistency
- Apply `focus:ring-2 focus:ring-primary` for accessibility
- Use `transition-all` for smooth hover states
- Maintain contrast ratios (WCAG 4.5:1 for text)
- Use the brand gradient sparingly for highlights

### DON'T:
- Hardcode colors outside the palette
- Use light backgrounds (system is dark-themed)
- Ignore focus states for accessibility
- Use pure black (#000000) - use #050505 or #1a1a1a

## Resources

- **Tailwind Config**: See `Services/yoizen-ui/tailwind.config.js`
- **Base Styles**: See `Services/yoizen-ui/src/index.css`
- **Brand Assets**: See `Services/yoizen-ui/public/`
- **Component Examples**: See `Services/yoizen-ui/src/components/`
