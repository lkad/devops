import { test, expect } from '@playwright/test'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    // Login first
    await page.goto('/login')
    await page.fill('input[name="username"]', 'admin')
    await page.fill('input[name="password"]', 'admin123')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')
  })

  test('stats cards render correctly', async ({ page }) => {
    await page.goto('/')

    // Check for stats cards
    const statsCards = page.locator('[class*="statsCard"], [class*="StatsCard"]')
    await expect(statsCards.first()).toBeVisible()
  })

  test('quick actions navigate correctly', async ({ page }) => {
    await page.goto('/')

    // Find and click quick actions
    const quickActions = page.locator('text=/View All Devices|View All Pipelines|View Logs|View Alerts/')
    const firstAction = quickActions.first()

    if (await firstAction.isVisible()) {
      await firstAction.click()
      // Should navigate somewhere
      await expect(page).not.toHaveURL(/\/login/)
    }
  })

  test('recent activity section shows', async ({ page }) => {
    await page.goto('/')

    // Check for recent activity section
    const recentActivity = page.locator('text=/Recent Activity|Latest Events/')
    await expect(recentActivity.first()).toBeVisible()
  })

  test('dashboard loads without errors', async ({ page }) => {
    const errors: string[] = []
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text())
      }
    })

    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Filter out expected warnings
    const actualErrors = errors.filter(
      (e) => !e.includes('Warning') && !e.includes('React does not recognize')
    )

    expect(actualErrors).toHaveLength(0)
  })
})