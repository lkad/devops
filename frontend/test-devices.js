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

    const bodyText = await page.textContent('body');

    const headings = await page.evaluate(() => {
      return Array.from(document.querySelectorAll('h1, h2, h3, h4, h5, h6')).map(e => e.textContent);
    });

    console.log('=== PAGE CONTENT ===');
    console.log('Body text length:', bodyText.length);
    console.log('Headings:', JSON.stringify(headings));

    // Check if devices are displayed
    const deviceCount = await page.evaluate(() => {
      // Check for table rows, list items, cards that might represent devices
      const tables = document.querySelectorAll('table tbody tr');
      const cards = document.querySelectorAll('[class*="device"], [class*="Device"]');
      const divs = document.querySelectorAll('div[class*="row"], div[class*="card"]');
      return {
        tableRows: tables.length,
        deviceElements: cards.length,
        cardDivs: divs.length,
        totalTextLength: document.body.textContent.length
      };
    });
    console.log('Device data:', JSON.stringify(deviceCount));

    console.log('\n=== CONSOLE ERRORS ===');
    consoleMessages.filter(m => m.type === 'error').forEach(m => console.log('ERROR:', m.text));

    console.log('\n=== PAGE ERRORS ===');
    pageErrors.forEach(e => console.log('PAGE ERROR:', e));

    await page.screenshot({ path: '/tmp/devices-page.png' });
    console.log('\nScreenshot saved to /tmp/devices-page.png');

  } catch (err) {
    console.log('Navigation error:', err.message);
  }

  await browser.close();
})();