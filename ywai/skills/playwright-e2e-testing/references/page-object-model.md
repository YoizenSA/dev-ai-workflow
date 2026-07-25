## Page Object Model

### Page Class Pattern

```typescript
// pages/LoginPage.ts
import { Page, Locator } from '@playwright/test';

export class LoginPage {
  readonly page: Page;
  readonly usernameInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page) {
    this.page = page;
    this.usernameInput = page.getByLabel('Username');
    this.passwordInput = page.getByLabel('Password');
    this.submitButton = page.getByRole('button', { name: 'Log in' });
    this.errorMessage = page.locator('.error-message');
  }

  async goto() {
    await this.page.goto('/login');
  }

  async login(username: string, password: string) {
    await this.usernameInput.fill(username);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }

  async expectErrorMessage(message: string) {
    await this.errorMessage.waitFor({ state: 'visible' });
    await expect(this.errorMessage).toHaveText(message);
  }
}

// pages/DashboardPage.ts
export class DashboardPage {
  readonly page: Page;
  readonly welcomeMessage: Locator;
  readonly logoutButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.welcomeMessage = page.locator('.welcome-message');
    this.logoutButton = page.getByRole('button', { name: 'Logout' });
  }

  async waitForLoad() {
    await this.welcomeMessage.waitFor({ state: 'visible' });
  }

  async logout() {
    await this.logoutButton.click();
  }
}

// tests/auth.spec.ts
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test('successful login flow', async ({ page }) => {
  const loginPage = new LoginPage(page);
  const dashboard = new DashboardPage(page);

  await loginPage.goto();
  await loginPage.login('testuser', 'password123');

  await dashboard.waitForLoad();
  await expect(dashboard.welcomeMessage).toContainText('Welcome');
});
```

### Component Pattern

```typescript
// components/NavigationComponent.ts
import { Page, Locator } from '@playwright/test';

export class NavigationComponent {
  readonly page: Page;
  readonly homeLink: Locator;
  readonly profileLink: Locator;
  readonly searchInput: Locator;

  constructor(page: Page) {
    this.page = page;
    const nav = page.locator('nav');
    this.homeLink = nav.getByRole('link', { name: 'Home' });
    this.profileLink = nav.getByRole('link', { name: 'Profile' });
    this.searchInput = nav.getByPlaceholder('Search...');
  }

  async navigateToProfile() {
    await this.profileLink.click();
  }

  async search(query: string) {
    await this.searchInput.fill(query);
    await this.searchInput.press('Enter');
  }
}
```

## Test Organization

### Custom Fixtures

```typescript
// fixtures/todos.ts
import { test as base } from '@playwright/test';

type TodoFixtures = {
  todoPage: TodoPage;
  createTodo: (title: string) => Promise<void>;
};

export const test = base.extend<TodoFixtures>({
  todoPage: async ({ page }, use) => {
    const todoPage = new TodoPage(page);
    await todoPage.goto();
    await use(todoPage);
  },

  createTodo: async ({ page }, use) => {
    const create = async (title: string) => {
      await page.fill('.new-todo', title);
      await page.press('.new-todo', 'Enter');
    };
    await use(create);
  },
});

// tests/todos.spec.ts
import { test } from '../fixtures/todos';

test('can create new todo', async ({ todoPage, createTodo }) => {
  await createTodo('Buy groceries');
  await expect(todoPage.todoItems).toHaveCount(1);
  await expect(todoPage.todoItems).toHaveText('Buy groceries');
});
```

### Test Tags and Filtering

```typescript
test('smoke test', { tag: '@smoke' }, async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle('Home');
});

test('regression test', { tag: ['@regression', '@critical'] }, async ({ page }) => {
  // Complex test
});

// Run: npx playwright test --grep @smoke
// Run: npx playwright test --grep-invert @slow
```
