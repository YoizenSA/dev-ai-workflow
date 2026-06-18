---
name: webapp-testing
description: "Comprehensive web application testing patterns with Playwright TypeScript, selectors, wait strategies, and best practices"
user-invocable: false
disable-model-invocation: true
progressive_disclosure:
  entry_point:
    summary: "Comprehensive web application testing patterns with Playwright TypeScript, selectors, wait strategies, and best practices"
    when_to_use: "When writing tests, implementing webapp-testing-patterns, or ensuring code quality."
    quick_start: "1. Review the core concepts below. 2. Apply patterns to your use case. 3. Follow best practices for implementation."
---
# Playwright Patterns Reference (TypeScript)

Complete guide to Playwright automation patterns, selectors, and best practices using TypeScript.

## Table of Contents

- [Selectors](#selectors)
- [Wait Strategies](#wait-strategies)
- [Element Interactions](#element-interactions)
- [Assertions](#assertions)
- [Test Organization](#test-organization)
- [Network Interception](#network-interception)
- [Screenshots and Videos](#screenshots-and-videos)
- [Debugging](#debugging)
- [Parallel Execution](#parallel-execution)

## Selectors

### Text Selectors
Most readable and maintainable approach when text is unique:

```typescript
await page.click('text=Login')
await page.click('text="Sign Up"')  // Exact match
await page.click('text=/log.*in/i')  // Regex, case-insensitive
```

### Role-Based Selectors
Semantic selectors based on ARIA roles:

```typescript
await page.click('role=button[name="Submit"]')
await page.fill('role=textbox[name="Email"]', 'user@example.com')
await page.click('role=link[name="Learn more"]')
await page.check('role=checkbox[name="Accept terms"]')
```

### CSS Selectors
Traditional CSS selectors for precise targeting:

```typescript
await page.click('#submit-button')
await page.fill('.email-input', 'user@example.com')
await page.click('button.primary')
await page.click('nav > ul > li:first-child')
```

### XPath Selectors
For complex DOM navigation:

```typescript
await page.click('xpath=//button[contains(text(), "Submit")]')
await page.click('xpath=//div[@class="modal"]//button[@type="submit"]')
```

### Data Attributes
Best practice for test-specific selectors:

```typescript
await page.click('[data-testid="submit-btn"]')
await page.fill('[data-test="email-input"]', 'test@example.com')
```

### Chaining Selectors
Combine selectors for precision:

```typescript
await page.locator('div.modal').locator('button.submit').click()
await page.locator('role=dialog').locator('text=Confirm').click()
```

### Selector Best Practices

**Priority order (most stable to least stable):**
1. `data-testid` attributes (most stable)
2. `role=` selectors (semantic, accessible)
3. `text=` selectors (readable, but text may change)
4. `id` attributes (stable if not dynamic)
5. CSS classes (less stable, may change with styling)
6. XPath (fragile, avoid if possible)

## Wait Strategies

### Load State Waits
Essential for dynamic applications:

```typescript
// Wait for network to be idle (most common)
await page.goto('http://localhost:3000')
await page.waitForLoadState('networkidle')

// Wait for DOM to be ready
await page.waitForLoadState('domcontentloaded')

// Wait for full load including images
await page.waitForLoadState('load')
```

### Element Waits
Wait for specific elements before interacting:

```typescript
// Wait for element to be visible
await page.waitForSelector('button.submit', { state: 'visible' })

// Wait for element to be hidden
await page.waitForSelector('.loading-spinner', { state: 'hidden' })

// Wait for element to exist in DOM (may not be visible)
await page.waitForSelector('.modal', { state: 'attached' })

// Wait for element to be removed from DOM
await page.waitForSelector('.error-message', { state: 'detached' })
```

### Timeout Waits
Fixed time delays (use sparingly):

```typescript
// Wait for animations to complete
await page.waitForTimeout(500)

// Wait for delayed content (better to use waitForSelector)
await page.waitForTimeout(2000)
```

### Custom Wait Conditions
Wait for JavaScript conditions:

```typescript
// Wait for custom JavaScript condition
await page.waitForFunction('() => document.querySelector(".data").innerText !== "Loading..."')

// Wait for variable to be set
await page.waitForFunction('() => window.appReady === true')
```

### Auto-Waiting
Playwright automatically waits for elements to be actionable:

```typescript
// These automatically wait for element to be:
// - Visible
// - Stable (not animating)
// - Enabled (not disabled)
// - Not obscured by other elements
await page.click('button.submit')  // Auto-waits
await page.fill('input.email', 'test@example.com')  // Auto-waits
```

## Element Interactions

### Clicking
```typescript
// Basic click
await page.click('button.submit')

// Click with options
await page.click('button.submit', { button: 'right' })  // Right-click
await page.click('button.submit', { clickCount: 2 })  // Double-click
await page.click('button.submit', { modifiers: ['Control'] })  // Ctrl+click

// Force click (bypass actionability checks)
await page.click('button.submit', { force: true })
```

### Filling Forms
```typescript
// Text inputs
await page.fill('input[name="email"]', 'user@example.com')
await page.type('input[name="search"]', 'query', { delay: 100 })  // Type with delay

// Clear then fill
await page.fill('input[name="email"]', '')
await page.fill('input[name="email"]', 'new@example.com')

// Press keys
await page.press('input[name="search"]', 'Enter')
await page.press('input[name="text"]', 'Control+A')
```

### Dropdowns and Selects
```typescript
// Select by label
await page.selectOption('select[name="country"]', { label: 'United States' })

// Select by value
await page.selectOption('select[name="country"]', { value: 'us' })

// Select by index
await page.selectOption('select[name="country"]', { index: 2 })

// Select multiple options
await page.selectOption('select[multiple]', ['option1', 'option2'])
```

### Checkboxes and Radio Buttons
```typescript
// Check a checkbox
await page.check('input[type="checkbox"]')

// Uncheck a checkbox
await page.uncheck('input[type="checkbox"]')

// Check a radio button
await page.check('input[value="option1"]')

// Toggle checkbox
if (await page.isChecked('input[type="checkbox"]')) {
  await page.uncheck('input[type="checkbox"]')
} else {
  await page.check('input[type="checkbox"]')
}
```

### File Uploads
```typescript
// Upload single file
await page.setInputFiles('input[type="file"]', '/path/to/file.pdf')

// Upload multiple files
await page.setInputFiles('input[type="file"]', ['/path/to/file1.pdf', '/path/to/file2.pdf'])

// Clear file input
await page.setInputFiles('input[type="file"]', [])
```

### Hover and Focus
```typescript
// Hover over element
await page.hover('button.tooltip-trigger')

// Focus element
await page.focus('input[name="email"]')

// Blur element
await page.evaluate('document.activeElement.blur()')
```

## Assertions

### Element Visibility
```typescript
import { expect } from '@playwright/test'

// Expect element to be visible
await expect(page.locator('button.submit')).toBeVisible()

// Expect element to be hidden
await expect(page.locator('.error-message')).toBeHidden()
```

### Text Content
```typescript
// Expect exact text
await expect(page.locator('.title')).toHaveText('Welcome')

// Expect partial text
await expect(page.locator('.message')).toContainText('success')

// Expect text matching pattern
await expect(page.locator('.code')).toHaveText(/\d{6}/)
```

### Element State
```typescript
// Expect element to be enabled/disabled
await expect(page.locator('button.submit')).toBeEnabled()
await expect(page.locator('button.submit')).toBeDisabled()

// Expect checkbox to be checked
await expect(page.locator('input[type="checkbox"]')).toBeChecked()

// Expect element to be editable
await expect(page.locator('input[name="email"]')).toBeEditable()
```

### Attributes and Values
```typescript
// Expect attribute value
await expect(page.locator('img')).toHaveAttribute('src', '/logo.png')

// Expect CSS class
await expect(page.locator('button')).toHaveClass('btn-primary')

// Expect input value
await expect(page.locator('input[name="email"]')).toHaveValue('user@example.com')
```

### Count and Collections
```typescript
// Expect specific count
await expect(page.locator('li')).toHaveCount(5)

// Get all elements and assert
const items = await page.locator('li').all()
expect(items.length).toBe(5)
```

## Test Organization

### Basic Test Structure
```typescript
import { test, expect } from '@playwright/test'

test('basic test example', async ({ page }) => {
  // Test logic here
  await page.goto('http://localhost:3000')
  await page.waitForLoadState('networkidle')
})
```

### Using Playwright Test (Recommended)
```typescript
import { test, expect } from '@playwright/test'

// Playwright Test provides built-in fixtures: page, browser, context
// No manual setup needed — test isolation is automatic

test('login test', async ({ page }) => {
  await page.goto('http://localhost:3000')
  await page.fill('input[name="email"]', 'user@example.com')
  await page.fill('input[name="password"]', 'password123')
  await page.click('button[type="submit"]')
  await expect(page.locator('.welcome-message')).toBeVisible()
})
```

### Test Grouping with Describe Blocks
```typescript
import { test, expect } from '@playwright/test'

test.describe('Authentication', () => {
  test('successful login', async ({ page }) => {
    // Test successful login
  })

  test('failed login', async ({ page }) => {
    // Test failed login
  })

  test('logout', async ({ page }) => {
    // Test logout
  })
})
```

### Setup and Teardown
```typescript
import { test, expect } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  // Setup - runs before each test
  await page.goto('http://localhost:3000')
  await page.waitForLoadState('networkidle')
})

test.afterEach(async ({ page }) => {
  // Teardown - runs after each test
  await page.evaluate(() => localStorage.clear())
})
```

## Network Interception

### Mock API Responses
```typescript
// Intercept and mock API response
await page.route('**/api/data', async (route) => {
  await route.fulfill({
    status: 200,
    body: JSON.stringify({ success: true, data: 'mocked' }),
    headers: { 'Content-Type': 'application/json' },
  })
})

await page.goto('http://localhost:3000')
```

### Block Resources
```typescript
// Block images and stylesheets for faster tests
await page.route('**/*.{png,jpg,jpeg,gif,svg,css}', (route) => route.abort())
```

### Wait for Network Responses
```typescript
// Wait for specific API call
const [response] = await Promise.all([
  page.waitForResponse('**/api/users'),
  page.click('button.load-users'),
])
expect(response.status()).toBe(200)
```

## Screenshots and Videos

### Screenshots
```typescript
// Full page screenshot
await page.screenshot({ path: '/tmp/screenshot.png', fullPage: true })

// Element screenshot
await page.locator('.modal').screenshot({ path: '/tmp/modal.png' })

// Screenshot with custom dimensions
await page.setViewportSize({ width: 1920, height: 1080 })
await page.screenshot({ path: '/tmp/desktop.png' })
```

### Video Recording
```typescript
// In playwright.config.ts:
// use: {
//   video: 'on-first-retry', // or 'on', 'retain-on-failure'
// }

// Or per test context:
const context = await browser.newContext({
  recordVideo: { dir: '/tmp/videos/' },
})
const page = await context.newPage()

// Perform actions...

await context.close()  // Video saved on close
```

## Debugging

### Pause Execution
```typescript
await page.pause()  // Opens Playwright Inspector
```

### Console Logs
```typescript
page.on('console', (msg) => {
  console.log(`[${msg.type()}] ${msg.text()}`)
})
```

### Slow Motion
```typescript
// In playwright.config.ts:
// use: {
//   launchOptions: {
//     slowMo: 1000,
//   },
// }

// Or inline:
const browser = await chromium.launch({ headless: false, slowMo: 1000 })
```

### Verbose Logging
```bash
# Set DEBUG environment variable
DEBUG=pw:api npx playwright test
```

## Parallel Execution

### Playwright Test Parallelism
```typescript
// playwright.config.ts
import { defineConfig } from '@playwright/test'

export default defineConfig({
  fullyParallel: true,
  workers: process.env.CI ? 1 : undefined,
})

// Run tests in parallel (default)
// npx playwright test

// Run with specific number of workers
// npx playwright test --workers=4
```

### Browser Context Isolation
```typescript
// Each test gets an isolated context (cookies, localStorage, etc.)
// Playwright Test handles this automatically via the `page` fixture.

// For manual context isolation:
import { test as base } from '@playwright/test'

const test = base.extend<{ context: BrowserContext }>({
  context: async ({ browser }, use) => {
    const context = await browser.newContext()
    await use(context)
    await context.close()
  },
})
```
