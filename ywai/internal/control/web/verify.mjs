import puppeteer from 'puppeteer';

(async () => {
  const browser = await puppeteer.launch({ headless: 'new' });
  const page = await browser.newPage();
  page.setViewport({ width: 1200, height: 900 });
  
  console.log('Navigating to http://localhost:5768/missions...');
  await page.goto('http://localhost:5768/missions', { waitUntil: 'networkidle2', timeout: 10000 });
  
  // Wait for the Missions page to load
  await page.waitForTimeout(2000);
  
  // Try to find and click "New Mission" button
  const buttons = await page.$$('button');
  let found = false;
  for (const btn of buttons) {
    const text = await page.evaluate(el => el.textContent, btn);
    if (text && (text.includes('New') || text.includes('new'))) {
      console.log(`Found button with text: "${text.trim()}", clicking...`);
      await btn.click();
      found = true;
      break;
    }
  }
  
  if (!found) {
    console.log('Button not found, checking page content...');
    const content = await page.content();
    if (content.includes('New Mission')) {
      console.log('Found "New Mission" text in page');
    }
  }
  
  // Wait for modal
  await page.waitForTimeout(2000);
  
  // Try to navigate to Step 4 by clicking the step indicator directly
  const stepButtons = await page.$$('.wizard-step');
  console.log(`Found ${stepButtons.length} step indicators`);
  
  if (stepButtons.length >= 4) {
    console.log('Clicking Step 4 directly...');
    // Try to find the "Review" step and click
    await page.click('.wizard-step:nth-child(4)', { timeout: 5000 }).catch(() => {
      console.log('Could not click step 4 directly');
    });
    await page.waitForTimeout(1000);
  }
  
  // Look for model selectors
  const modelSelects = await page.$$('select[id*="model"]');
  console.log(`Found ${modelSelects.length} model selectors`);
  
  if (modelSelects.length > 0) {
    const ids = await page.$$eval('select[id*="model"]', els => els.map(el => el.id));
    console.log(`Model selector IDs: ${ids.join(', ')}`);
  }
  
  // Check for optgroups (grouped by provider)
  const optgroups = await page.$$('optgroup');
  console.log(`Found ${optgroups.length} optgroup elements (provider grouping)`);
  
  if (optgroups.length > 0) {
    const labels = await page.$$eval('optgroup', els => els.map(el => el.label));
    console.log(`Provider groups: ${labels.join(', ')}`);
  }
  
  // Count total options across all selects
  const allOptions = await page.$$('select[id*="model"] option');
  console.log(`Total model options: ${allOptions.length}`);
  
  // Take screenshot
  console.log('Taking screenshot...');
  await page.screenshot({ path: '/tmp/mission-wizard.png' });
  console.log('✅ Screenshot saved to /tmp/mission-wizard.png');
  
  // Print what we found
  if (modelSelects.length === 4) {
    console.log('✅ Found 4 model selectors (goal, plan, plan-refine, worker)');
  } else {
    console.log(`⚠️  Found ${modelSelects.length} model selectors, expected 4`);
  }
  
  if (optgroups.length > 0) {
    console.log(`✅ Found provider grouping (${optgroups.length} optgroups)`);
  }
  
  if (allOptions.length > 10) {
    console.log(`✅ Found ${allOptions.length} total model options (vs 3 before fix)`);
  }
  
  await browser.close();
})().catch(console.error);
