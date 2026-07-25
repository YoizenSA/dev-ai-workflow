## Locator Strategies

### Best Practice: Role-based Locators

```typescript
import { test, expect } from '@playwright/test';

test('accessible locators', async ({ page }) => {
  await page.goto('/form');

  // By role (BEST - accessible and stable)
  await page.getByRole('button', { name: 'Submit' }).click();
  await page.getByRole('textbox', { name: 'Email' }).fill('user@example.com');
  await page.getByRole('checkbox', { name: 'Subscribe' }).check();
  await page.getByRole('link', { name: 'Learn more' }).click();

  // By label (good for forms)
  await page.getByLabel('Password').fill('secret123');

  // By placeholder
  await page.getByPlaceholder('Search...').fill('query');

  // By text
  await page.getByText('Welcome back').click();
  await page.getByText(/hello/i).isVisible();

  // By test ID (good for dynamic content)
  await page.getByTestId('user-profile').click();

  // By title
  await page.getByTitle('Close dialog').click();

  // By alt text (images)
  await page.getByAltText('User avatar').click();
});
```

### CSS and XPath Locators

```typescript
test('CSS and XPath locators', async ({ page }) => {
  // CSS selectors
  await page.locator('button.primary').click();
  await page.locator('#user-menu').click();
  await page.locator('[data-testid="submit-btn"]').click();
  await page.locator('div.card:first-child').click();

  // XPath (use sparingly)
  await page.locator('xpath=//button[contains(text(), "Submit")]').click();

  // Chaining locators
  const form = page.locator('form#login-form');
  await form.locator('input[name="email"]').fill('user@example.com');
  await form.locator('button[type="submit"]').click();

  // Filter locators
  await page.getByRole('listitem')
    .filter({ hasText: 'Product 1' })
    .getByRole('button', { name: 'Add to cart' })
    .click();
});
```

## User Interactions

### Form Interactions

```typescript
test('form interactions', async ({ page }) => {
  await page.goto('/form');

  // Text inputs
  await page.fill('input[name="email"]', 'user@example.com');
  await page.type('textarea[name="message"]', 'Hello', { delay: 100 });

  // Checkboxes
  await page.check('input[type="checkbox"][name="subscribe"]');
  await page.uncheck('input[type="checkbox"][name="spam"]');

  // Radio buttons
  await page.check('input[type="radio"][value="option1"]');

  // Select dropdowns
  await page.selectOption('select[name="country"]', 'US');
  await page.selectOption('select[name="color"]', { label: 'Blue' });
  await page.selectOption('select[name="size"]', { value: 'large' });

  // Multi-select
  await page.selectOption('select[multiple]', ['value1', 'value2']);

  // File uploads
  await page.setInputFiles('input[type="file"]', 'path/to/file.pdf');
  await page.setInputFiles('input[type="file"]', [
    'file1.jpg',
    'file2.jpg'
  ]);

  // Clear file input
  await page.setInputFiles('input[type="file"]', []);
});
```

### Mouse and Keyboard

```typescript
test('mouse and keyboard interactions', async ({ page }) => {
  // Click variations
  await page.click('button');
  await page.dblclick('button'); // Double click
  await page.click('button', { button: 'right' }); // Right click
  await page.click('button', { modifiers: ['Shift'] }); // Shift+click

  // Hover
  await page.hover('.tooltip-trigger');
  await expect(page.locator('.tooltip')).toBeVisible();

  // Drag and drop
  await page.dragAndDrop('#draggable', '#droppable');

  // Keyboard
  await page.keyboard.press('Enter');
  await page.keyboard.press('Control+A');
  await page.keyboard.type('Hello World');
  await page.keyboard.down('Shift');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.up('Shift');

  // Focus
  await page.focus('input[name="email"]');
  await page.fill('input[name="email"]', 'test@example.com');
});
```

### Waiting Strategies

```typescript
test('waiting strategies', async ({ page }) => {
  // Wait for element
  await page.waitForSelector('.dynamic-content');
  await page.waitForSelector('.modal', { state: 'visible' });
  await page.waitForSelector('.loading', { state: 'hidden' });

  // Wait for load state
  await page.waitForLoadState('load');
  await page.waitForLoadState('domcontentloaded');
  await page.waitForLoadState('networkidle');

  // Wait for URL
  await page.waitForURL('**/dashboard');
  await page.waitForURL(/\/product\/\d+/);

  // Wait for function
  await page.waitForFunction(() => {
    return document.querySelectorAll('.item').length > 5;
  });

  // Wait for timeout (avoid if possible)
  await page.waitForTimeout(1000);

  // Wait for event
  await page.waitForEvent('load');
  await page.waitForEvent('popup');
});
```

## Assertions

### Common Assertions

```typescript
import { test, expect } from '@playwright/test';

test('assertions', async ({ page }) => {
  await page.goto('/dashboard');

  // Visibility
  await expect(page.locator('.header')).toBeVisible();
  await expect(page.locator('.loading')).toBeHidden();
  await expect(page.locator('.optional')).not.toBeVisible();

  // Text content
  await expect(page.locator('h1')).toHaveText('Dashboard');
  await expect(page.locator('h1')).toContainText('Dash');
  await expect(page.locator('.message')).toHaveText(/welcome/i);

  // Attributes
  await expect(page.locator('button')).toBeEnabled();
  await expect(page.locator('button')).toBeDisabled();
  await expect(page.locator('input')).toHaveAttribute('type', 'email');
  await expect(page.locator('input')).toHaveValue('test@example.com');

  // CSS
  await expect(page.locator('.button')).toHaveClass('btn-primary');
  await expect(page.locator('.button')).toHaveClass(/btn-/);
  await expect(page.locator('.element')).toHaveCSS('color', 'rgb(255, 0, 0)');

  // Count
  await expect(page.locator('.item')).toHaveCount(5);

  // URL and title
  await expect(page).toHaveURL('http://localhost:3000/dashboard');
  await expect(page).toHaveURL(/dashboard$/);
  await expect(page).toHaveTitle('Dashboard - My App');
  await expect(page).toHaveTitle(/Dashboard/);

  // Screenshot comparison
  await expect(page).toHaveScreenshot('dashboard.png');
  await expect(page.locator('.widget')).toHaveScreenshot('widget.png');
});
```

### Custom Assertions

```typescript
test('custom matchers', async ({ page }) => {
  // Soft assertions (continue test on failure)
  await expect.soft(page.locator('.title')).toHaveText('Welcome');
  await expect.soft(page.locator('.subtitle')).toBeVisible();

  // Multiple elements
  const items = page.locator('.item');
  await expect(items).toHaveCount(3);
  await expect(items.nth(0)).toContainText('First');
  await expect(items.nth(1)).toContainText('Second');

  // Poll assertions
  await expect(async () => {
    const response = await page.request.get('/api/status');
    expect(response.ok()).toBeTruthy();
  }).toPass({
    timeout: 10000,
    intervals: [1000, 2000, 5000],
  });
});
```
