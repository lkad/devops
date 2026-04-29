import { test, expect } from '@playwright/test'

test.describe('Login Flow', () => {
  test('shows login page', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('heading', { name: /sign in/i })).toBeVisible()
  })

  test('login with valid credentials', async ({ page }) => {
    await page.goto('/login')

    // Fill in credentials - using name attributes as per spec
    await page.fill('input[name="username"]', 'admin')
    await page.fill('input[name="password"]', 'admin123')

    // Submit form
    await page.click('button[type="submit"]')

    // Should redirect to dashboard
    await expect(page).toHaveURL('/')
  })

  test('login shows error with invalid credentials', async ({ page }) => {
    await page.goto('/login')

    await page.fill('input[name="username"]', 'admin')
    await page.fill('input[name="password"]', 'wrongpassword')

    await page.click('button[type="submit"]')

    // Should show error message
    await expect(page.getByText(/invalid credentials/i)).toBeVisible()
  })

  test('redirects to login when accessing protected route', async ({ page }) => {
    await page.goto('/devices')

    // Should redirect to login
    await expect(page).toHaveURL(/\/login/)
  })
})

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    // Login first
    await page.goto('/login')
    await page.fill('input[name="username"]', 'admin')
    await page.fill('input[name="password"]', 'admin123')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')
  })

  test('navigation to devices page', async ({ page }) => {
    await page.click('text=Devices')
    await expect(page).toHaveURL('/devices')
  })

  test('navigation to pipelines page', async ({ page }) => {
    await page.click('text=Pipelines')
    await expect(page).toHaveURL('/pipelines')
  })

  test('navigation to logs page', async ({ page }) => {
    await page.click('text=Logs')
    await expect(page).toHaveURL('/logs')
  })

  test('navigation to alerts page', async ({ page }) => {
    await page.click('text=Alerts')
    await expect(page).toHaveURL('/alerts')
  })

  test('logout works correctly', async ({ page }) => {
    await page.click('text=Logout')
    await expect(page).toHaveURL('/login')
  })
})