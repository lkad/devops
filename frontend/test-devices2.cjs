const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();

  const consoleMessages = [];
  page.on('console', msg => {
    consoleMessages.push({ type: msg.type(), text: msg.text() });
  });

  const pageErrors = [];
  page.on('pageerror', err => {
    pageErrors.push(err.message);
  });

  try {
    await page.goto('http://localhost:3000/devices', { waitUntil: 'networkidle', timeout: 15000 });

    // Wait a bit for React to render
    await page.waitForTimeout(2000);

    const bodyText = await page.textContent('body');

    console.log('=== PAGE CONTENT ===');
    console.log('Body text:', bodyText.substring(0, 500));

    // Get page HTML structure
    const htmlPreview = await page.evaluate(() => {
      const root = document.getElementById('root');
      if (root) {
        return root.innerHTML.substring(0, 1000);
      }
      return 'No #root element found';
    });
    console.log('\n=== ROOT HTML ===');
    console.log(htmlPreview);

    console.log('\n=== CONSOLE ERRORS ===');
    if (consoleMessages.filter(m => m.type === 'error').length === 0) {
      console.log('No console errors');
    }
    consoleMessages.filter(m => m.type === 'error').forEach(m => console.log('ERROR:', m.text));

    console.log('\n=== PAGE ERRORS ===');
    if (pageErrors.length === 0) {
      console.log('No page errors');
    }
    pageErrors.forEach(e => console.log('PAGE ERROR:', e));

    await page.screenshot({ path: '/tmp/devices-page.png', fullPage: true });
    console.log('\nScreenshot saved to /tmp/devices-page.png');

  } catch (err) {
    console.log('Navigation error:', err.message);
  }

  await browser.close();
})();