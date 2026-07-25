## Common Patterns

### Multi-Page Scenarios

```typescript
test('popup handling', async ({ page, context }) => {
  // Listen for new page
  const popupPromise = context.waitForEvent('page');
  await page.click('a[target="_blank"]');
  const popup = await popupPromise;

  await popup.waitForLoadState();
  await expect(popup).toHaveTitle('New Window');
  await popup.close();
});
```

### Conditional Logic

```typescript
test('handle optional elements', async ({ page }) => {
  await page.goto('/');

  // Close modal if present
  const modal = page.locator('.modal');
  if (await modal.isVisible()) {
    await page.click('.modal .close-button');
  }

  // Or use count
  const cookieBanner = page.locator('.cookie-banner');
  if ((await cookieBanner.count()) > 0) {
    await page.click('.accept-cookies');
  }
});
```

### Data-Driven Tests

```typescript
const testCases = [
  { input: 'hello', expected: 'HELLO' },
  { input: 'World', expected: 'WORLD' },
  { input: '123', expected: '123' },
];

for (const { input, expected } of testCases) {
  test(`transforms "${input}" to "${expected}"`, async ({ page }) => {
    await page.goto('/transform');
    await page.fill('input', input);
    await page.click('button');
    await expect(page.locator('.result')).toHaveText(expected);
  });
}
```
