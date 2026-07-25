## Visual Testing

### Screenshot Comparison

```typescript
test('visual regression', async ({ page }) => {
  await page.goto('/dashboard');

  // Full page screenshot
  await expect(page).toHaveScreenshot('dashboard.png', {
    maxDiffPixels: 100,
  });

  // Element screenshot
  await expect(page.locator('.widget')).toHaveScreenshot('widget.png');

  // Full page with scroll
  await expect(page).toHaveScreenshot('full-page.png', {
    fullPage: true,
  });

  // Mask dynamic elements
  await expect(page).toHaveScreenshot('masked.png', {
    mask: [page.locator('.timestamp'), page.locator('.avatar')],
  });

  // Custom threshold
  await expect(page).toHaveScreenshot('comparison.png', {
    maxDiffPixelRatio: 0.05, // 5% difference allowed
  });
});
```

### Video and Trace

```typescript
// playwright.config.ts
export default defineConfig({
  use: {
    video: 'retain-on-failure',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
});

// Programmatic video
test('record video', async ({ page }) => {
  await page.goto('/');
  // Test actions...

  // Video saved automatically to test-results/
});

// View trace: npx playwright show-trace trace.zip
```

## Parallel Execution

### Test Sharding

```typescript
// playwright.config.ts
export default defineConfig({
  fullyParallel: true,
  workers: process.env.CI ? 4 : undefined,
});

// Run shards in CI
// npx playwright test --shard=1/4
// npx playwright test --shard=2/4
// npx playwright test --shard=3/4
// npx playwright test --shard=4/4
```

### Serial Tests

```typescript
test.describe.configure({ mode: 'serial' });

test.describe('order matters', () => {
  let orderId: string;

  test('create order', async ({ page }) => {
    // Create order
    orderId = await createOrder(page);
  });

  test('verify order', async ({ page }) => {
    // Use orderId from previous test
    await verifyOrder(page, orderId);
  });
});
```

## Debugging

### UI Mode

```bash
# Interactive debugging
npx playwright test --ui

# Debug specific test
npx playwright test --debug login.spec.ts

# Step through test
npx playwright test --headed --slow-mo=1000
```

### Trace Viewer

```typescript
// Generate trace
test('with trace', async ({ page }) => {
  await page.context().tracing.start({ screenshots: true, snapshots: true });

  // Test actions
  await page.goto('/');

  await page.context().tracing.stop({ path: 'trace.zip' });
});

// View: npx playwright show-trace trace.zip
```

### Console Logs

```typescript
test('capture console', async ({ page }) => {
  page.on('console', msg => console.log(`Browser: ${msg.text()}`));
  page.on('pageerror', error => console.error(`Error: ${error.message}`));

  await page.goto('/');
});
```
