## Authentication Patterns

### Storage State Pattern

```typescript
// auth.setup.ts - Run once to save auth state
import { test as setup } from '@playwright/test';

const authFile = 'playwright/.auth/user.json';

setup('authenticate', async ({ page }) => {
  await page.goto('/login');
  await page.fill('[name="username"]', 'testuser');
  await page.fill('[name="password"]', 'password123');
  await page.click('button[type="submit"]');

  await page.waitForURL('/dashboard');

  // Save authentication state
  await page.context().storageState({ path: authFile });
});

// playwright.config.ts
export default defineConfig({
  projects: [
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: authFile,
      },
      dependencies: ['setup'],
    },
  ],
});

// tests/dashboard.spec.ts - Already authenticated
test('view dashboard', async ({ page }) => {
  await page.goto('/dashboard');
  // Already logged in!
  await expect(page.locator('.user-menu')).toBeVisible();
});
```

### Multiple User Roles

```typescript
// fixtures/auth.ts
import { test as base } from '@playwright/test';

type Fixtures = {
  adminPage: Page;
  userPage: Page;
};

export const test = base.extend<Fixtures>({
  adminPage: async ({ browser }, use) => {
    const context = await browser.newContext({
      storageState: 'playwright/.auth/admin.json',
    });
    const page = await context.newPage();
    await use(page);
    await context.close();
  },

  userPage: async ({ browser }, use) => {
    const context = await browser.newContext({
      storageState: 'playwright/.auth/user.json',
    });
    const page = await context.newPage();
    await use(page);
    await context.close();
  },
});

// tests/permissions.spec.ts
import { test } from '../fixtures/auth';

test('admin can access admin panel', async ({ adminPage }) => {
  await adminPage.goto('/admin');
  await expect(adminPage.locator('.admin-panel')).toBeVisible();
});

test('regular user cannot access admin panel', async ({ userPage }) => {
  await userPage.goto('/admin');
  await expect(userPage.locator('.access-denied')).toBeVisible();
});
```

## Network Control

### Request Mocking

```typescript
test('mock API responses', async ({ page }) => {
  // Mock API response
  await page.route('**/api/users', route => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        users: [
          { id: 1, name: 'John Doe' },
          { id: 2, name: 'Jane Smith' },
        ],
      }),
    });
  });

  await page.goto('/users');
  await expect(page.locator('.user-list')).toContainText('John Doe');
});

test('mock with conditions', async ({ page }) => {
  await page.route('**/api/**', route => {
    const url = route.request().url();

    if (url.includes('/users/1')) {
      route.fulfill({
        status: 200,
        body: JSON.stringify({ id: 1, name: 'Test User' }),
      });
    } else if (url.includes('/users')) {
      route.fulfill({
        status: 200,
        body: JSON.stringify({ users: [] }),
      });
    } else {
      route.continue();
    }
  });
});

test('simulate network errors', async ({ page }) => {
  await page.route('**/api/data', route => {
    route.abort('failed');
  });

  await page.goto('/data');
  await expect(page.locator('.error-message')).toBeVisible();
});
```

### Request Interception

```typescript
test('intercept and modify requests', async ({ page }) => {
  // Modify request headers
  await page.route('**/api/**', route => {
    const headers = route.request().headers();
    route.continue({
      headers: {
        ...headers,
        'X-Custom-Header': 'test-value',
      },
    });
  });

  // Modify POST data
  await page.route('**/api/submit', route => {
    const postData = route.request().postDataJSON();
    route.continue({
      postData: JSON.stringify({
        ...postData,
        timestamp: Date.now(),
      }),
    });
  });
});

test('wait for API response', async ({ page }) => {
  // Wait for specific request
  const responsePromise = page.waitForResponse('**/api/users');
  await page.click('button#load-users');
  const response = await responsePromise;

  expect(response.status()).toBe(200);
  const data = await response.json();
  expect(data.users).toHaveLength(10);
});
```
