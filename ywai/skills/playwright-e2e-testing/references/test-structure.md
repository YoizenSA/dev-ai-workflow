## Fundamentals

### Basic Test Structure

```typescript
import { test, expect } from '@playwright/test';

test('basic test', async ({ page }) => {
  await page.goto('https://example.com');

  // Wait for element and check visibility
  const title = page.locator('h1');
  await expect(title).toBeVisible();
  await expect(title).toHaveText('Example Domain');

  // Get page title
  await expect(page).toHaveTitle(/Example/);
});

test.describe('User authentication', () => {
  test('should login successfully', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[name="username"]', 'testuser');
    await page.fill('[name="password"]', 'password123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('.welcome-message')).toContainText('Welcome');
  });

  test('should show error for invalid credentials', async ({ page }) => {
    await page.goto('/login');
    await page.fill('[name="username"]', 'invalid');
    await page.fill('[name="password"]', 'wrong');
    await page.click('button[type="submit"]');

    await expect(page.locator('.error-message')).toBeVisible();
    await expect(page.locator('.error-message')).toHaveText('Invalid credentials');
  });
});
```

### Test Hooks

```typescript
import { test, expect } from '@playwright/test';

test.describe('Dashboard tests', () => {
  test.beforeEach(async ({ page }) => {
    // Run before each test
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');
  });

  test.afterEach(async ({ page }) => {
    // Cleanup after each test
    await page.close();
  });

  test.beforeAll(async ({ browser }) => {
    // Run once before all tests in describe block
    console.log('Starting test suite');
  });

  test.afterAll(async ({ browser }) => {
    // Run once after all tests
    console.log('Test suite complete');
  });

  test('displays user data', async ({ page }) => {
    await expect(page.locator('.user-name')).toBeVisible();
  });
});
```
