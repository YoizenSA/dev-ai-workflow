---
name: condition-based-waiting
description: Anti-flaky test patterns using condition-based waiting instead of fixed timeouts. Trigger: flaky tests, waitForTimeout, race conditions, test stability, async waiting.
---

# Condition-Based Waiting

Anti-flaky test patterns using condition-based waiting instead of fixed timeouts.

## Quick Reference

| Pattern | Use When |
|---------|----------|
| `locator.waitFor()` | Element state changes |
| `expect(locator).toBeVisible()` | Visibility assertions |
| `page.waitForFunction()` | Custom JS conditions |
| `expect.poll()` | Repeated value checks |
| `page.waitForLoadState()` | Page lifecycle events |

## Core Principle

**Wait for conditions, not time.**

```typescript
// ❌ BAD: Fixed timeout
await page.waitForTimeout(2000);

// ✅ GOOD: Condition-based
await page.locator('.result').waitFor({ state: 'visible' });
await expect(page.locator('.result')).toBeVisible();
```

## Patterns

### 1. Element State Waiting

```typescript
// Wait for element to be attached to DOM
await page.locator('.content').waitFor({ state: 'attached' });

// Wait for element to be visible
await page.locator('.modal').waitFor({ state: 'visible' });

// Wait for element to be hidden
await page.locator('.loading').waitFor({ state: 'hidden' });

// With custom timeout
await page.locator('.slow-element').waitFor({
  state: 'visible',
  timeout: 10000
});
```

### 2. Assertion-Based Waiting

```typescript
// expect() auto-waits for condition
await expect(page.locator('.status')).toHaveText('Complete');
await expect(page.locator('.count')).toHaveText('5');
await expect(page.locator('.item')).toHaveCount(3);

// Negation assertions (wait for NOT)
await expect(page.locator('.loading')).not.toBeVisible();
await expect(page.locator('.error')).not.toBeAttached();

// Custom timeout on assertions
await expect(page.locator('.result')).toBeVisible({ timeout: 10000 });
```

### 3. Custom Condition Waiting

```typescript
// Wait for arbitrary JavaScript condition
await page.waitForFunction(() => {
  return document.querySelectorAll('.item').length >= 5;
});

// Wait with arguments
const minCount = 5;
await page.waitForFunction(
  (count) => document.querySelectorAll('.item').length >= count,
  minCount
);

// Wait for element state via JS
await page.waitForFunction(
  (selector) => document.querySelector(selector)?.textContent?.includes('Done'),
  '.status'
);
```

### 4. Polling Pattern

```typescript
// Poll for value changes
await expect.poll(async () => {
  const response = await page.request.get('/api/status');
  return await response.text();
}).toBe('ready');

// Poll with options
await expect.poll(async () => {
  return await page.locator('.count').textContent();
}, {
  message: 'Item count should increase',
  timeout: 15000,
  intervals: [500, 1000, 2000] // Custom retry intervals
}).toBe('10');
```

### 5. Network-Based Waiting

```typescript
// Wait for specific API response
const responsePromise = page.waitForResponse('**/api/data');
await page.locator('.load-button').click();
const response = await responsePromise;

// Wait for response and verify
await Promise.all([
  page.waitForResponse(resp =>
    resp.url().includes('/api/data') && resp.status() === 200
  ),
  page.locator('.submit').click()
]);

// Wait for network idle (use sparingly)
await page.waitForLoadState('networkidle');
```

### 6. Navigation Waiting

```typescript
// Wait for page load
await page.goto('/dashboard');
await page.waitForLoadState('domcontentloaded');

// Wait for specific load state
await page.waitForLoadState('load'); // All resources loaded
await page.waitForLoadState('networkidle'); // No network for 500ms

// Wait for URL change
await page.waitForURL('**/dashboard');
await page.waitForURL(/\/dashboard/);
```

### 7. Download and File Waiting

```typescript
// Wait for download
const downloadPromise = page.waitForEvent('download');
await page.locator('.download-btn').click();
const download = await downloadPromise;

// Wait for file input
const fileChooserPromise = page.waitForEvent('filechooser');
await page.locator('.upload-btn').click();
const fileChooser = await fileChooserPromise;
await fileChooser.setFiles('document.pdf');
```

### 8. Dialog and Popup Waiting

```typescript
// Wait for dialog
page.on('dialog', async dialog => {
  expect(dialog.message()).toBe('Are you sure?');
  await dialog.accept();
});
await page.locator('.delete-btn').click();

// Wait for popup
const popupPromise = page.waitForEvent('popup');
await page.locator('.open-window').click();
const popup = await popupPromise;
await popup.waitForLoadState();
```

## Anti-Patterns to Avoid

### ❌ Fixed Timeouts

```typescript
// BAD: Arbitrary wait
await page.waitForTimeout(2000);
await page.waitForTimeout(5000);

// GOOD: Wait for condition
await expect(page.locator('.result')).toBeVisible();
```

### ❌ Busy Waiting

```typescript
// BAD: Manual polling loop
while (await page.locator('.loading').isVisible()) {
  await page.waitForTimeout(100);
}

// GOOD: Built-in waiting
await page.locator('.loading').waitFor({ state: 'hidden' });
await expect(page.locator('.loading')).not.toBeVisible();
```

### ❌ Missing Await

```typescript
// BAD: Missing await (race condition)
page.locator('.submit').click();
expect(page.locator('.result')).toBeVisible();

// GOOD: Properly awaited
await page.locator('.submit').click();
await expect(page.locator('.result')).toBeVisible();
```

### ❌ Wrong Wait Target

```typescript
// BAD: Waiting for container when content matters
await page.locator('.container').waitFor();

// GOOD: Wait for actual content
await page.locator('.container .item').first().waitFor();
```

## Decision Matrix

```
Need to wait for...          → Use this pattern
─────────────────────────────────────────────────
Element visible/hidden       → locator.waitFor({ state })
Element text/value           → expect(locator).toHaveText()
Element count                → expect(locator).toHaveCount()
JS condition                 → page.waitForFunction()
API response                 → page.waitForResponse()
Page navigation              → page.waitForURL() / waitForLoadState()
Download/popup               → page.waitForEvent()
Repeated value check         → expect.poll()
Element NOT present          → expect(locator).not.toBeAttached()
```

## Integration with Test Structure

```typescript
import { test, expect } from '@playwright/test';

test.describe('Feature with proper waiting', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/app');
    await page.waitForLoadState('domcontentloaded');
  });

  test('loads data and displays results', async ({ page }) => {
    // Trigger action
    await page.locator('.load-data').click();

    // Wait for loading to finish
    await expect(page.locator('.loading')).not.toBeVisible();

    // Wait for results
    await expect(page.locator('.result-item')).toHaveCount(5);

    // Verify content
    await expect(page.locator('.result-item').first()).toHaveText(/expected/);
  });

  test('handles async operations', async ({ page }) => {
    // Setup response listener
    const responsePromise = page.waitForResponse('**/api/data');

    // Trigger action
    await page.locator('.fetch-btn').click();

    // Wait for and verify response
    const response = await responsePromise;
    expect(response.status()).toBe(200);

    // Wait for UI update
    await expect(page.locator('.data-display')).toBeVisible();
  });
});
```

## References

- [Playwright Auto-waiting](https://playwright.dev/docs/actionability)
- [Playwright Assertions](https://playwright.dev/docs/test-assertions)
- [Playwright Events](https://playwright.dev/docs/events)
