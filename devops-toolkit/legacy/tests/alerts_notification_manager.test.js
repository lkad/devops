/**
 * Tests for Alerts Notification Manager
 * Covers: channel management, rate limiting, alert triggering
 */

describe('AlertNotificationManager', () => {
  let alertsManager;

  beforeEach(() => {
    // Get the singleton instance
    alertsManager = require('../alerts_notification_manager');
    // Reset singleton state between tests to prevent pollution
    alertsManager.channels.clear();
    alertsManager.alertHistory = [];
    alertsManager.rateLimits.clear();
  });

  afterEach(() => {
    // Clean up channels after each test
    alertsManager.channels.clear();
    alertsManager.alertHistory = [];
    alertsManager.rateLimits.clear();
  });

  describe('Channel Management', () => {
    it('should add a webhook channel', () => {
      const result = alertsManager.addChannel('webhook_test_' + Date.now(), {
        type: 'webhook',
        url: 'https://example.com/webhook'
      });

      expect(result).toBeUndefined(); // addChannel doesn't return
      expect(alertsManager.channels.size).toBeGreaterThan(0);
    });

    it('should add a slack channel', () => {
      alertsManager.addChannel('slack_test_' + Date.now(), {
        type: 'slack',
        webhookUrl: 'https://hooks.slack.com/test'
      });

      expect(alertsManager.channels.size).toBeGreaterThan(0);
    });

    it('should add an email channel', () => {
      alertsManager.addChannel('email_test_' + Date.now(), {
        type: 'email',
        recipients: 'test@example.com'
      });

      expect(alertsManager.channels.size).toBeGreaterThan(0);
    });

    it('should add a log channel', () => {
      alertsManager.addChannel('log_test_' + Date.now(), {
        type: 'log',
        enabled: true
      });

      expect(alertsManager.channels.size).toBeGreaterThan(0);
    });

    it('should remove a channel', () => {
      const name = 'to_remove_' + Date.now();
      alertsManager.addChannel(name, { type: 'log' });
      alertsManager.removeChannel(name);

      expect(alertsManager.channels.has(name)).toBe(false);
    });

    it('should get all channels', () => {
      const channels = alertsManager.getChannels();
      expect(Array.isArray(channels)).toBe(true);
    });
  });

  describe('Rate Limiting', () => {
    it('should allow first request', () => {
      const result = alertsManager.checkRateLimit('new_alert_' + Date.now());
      expect(result).toBe(true);
    });

    it('should allow subsequent requests under limit', () => {
      const name = 'rate_test_' + Date.now();
      alertsManager.checkRateLimit(name);
      alertsManager.checkRateLimit(name);
      const result = alertsManager.checkRateLimit(name);
      expect(result).toBe(true);
    });

    it('should block when rate limit exceeded', () => {
      const name = 'exceeded_' + Date.now();
      for (let i = 0; i < alertsManager.maxAlertsPerWindow; i++) {
        alertsManager.checkRateLimit(name);
      }
      const result = alertsManager.checkRateLimit(name);
      expect(result).toBe(false);
    });
  });

  describe('Alert Triggering', () => {
    it('should trigger alert', async () => {
      const alert = {
        name: 'Test Alert',
        severity: 'warning',
        message: 'Test!'
      };

      const result = await alertsManager.triggerAlert(alert);
      expect(result).toBeDefined();
    });

    it('should add to history on trigger', async () => {
      const initialHistorySize = alertsManager.alertHistory.length;
      const alert = {
        name: 'History Test',
        severity: 'error',
        message: 'Error!'
      };

      await alertsManager.triggerAlert(alert);
      expect(alertsManager.alertHistory.length).toBeGreaterThan(initialHistorySize);
    });

    it('should rate limit alerts', () => {
      const name = 'Rate Limited ' + Date.now();

      // Fill rate limit directly via checkRateLimit
      for (let i = 0; i < alertsManager.maxAlertsPerWindow; i++) {
        alertsManager.checkRateLimit(name);
      }

      const result = alertsManager.checkRateLimit(name);
      expect(result).toBe(false);
    });
  });

  describe('History', () => {
    it('should add to history', () => {
      const initialLength = alertsManager.alertHistory.length;
      alertsManager.addToHistory({ name: 'Test', severity: 'info', message: 'test' });
      expect(alertsManager.alertHistory.length).toBeGreaterThan(initialLength);
    });

    it('should filter history by severity', () => {
      alertsManager.alertHistory = [];
      alertsManager.addToHistory({ name: 'Info Alert', severity: 'info', message: '', timestamp: new Date().toISOString() });
      alertsManager.addToHistory({ name: 'Error Alert', severity: 'critical', message: '', timestamp: new Date().toISOString() });

      const errors = alertsManager.getHistory({ severity: 'critical' });
      expect(errors.every(h => h.severity === 'critical')).toBe(true);
    });

    it('should filter by time', () => {
      const now = new Date();
      alertsManager.alertHistory = [
        { name: 'Old', severity: 'info', message: '', timestamp: new Date(now - 3600000).toISOString() },
        { name: 'New', severity: 'info', message: '', timestamp: now.toISOString() }
      ];

      const filtered = alertsManager.getHistory({ since: new Date(now - 1800000).toISOString() });
      expect(filtered.some(h => h.name === 'New')).toBe(true);
    });

    it('should limit history results', () => {
      alertsManager.alertHistory = [];
      for (let i = 0; i < 20; i++) {
        alertsManager.addToHistory({ name: `Alert ${i}`, severity: 'info', message: '' });
      }

      const history = alertsManager.getHistory({ limit: 10 });
      expect(history.length).toBe(10);
    });
  });

  describe('Stats', () => {
    it('should get stats', () => {
      const initialCount = alertsManager.alertHistory.length;
      alertsManager.addToHistory({ name: 'Test', severity: 'critical', message: '' });

      const stats = alertsManager.getStats();
      expect(stats).toBeDefined();
      expect(stats.total).toBeGreaterThan(initialCount);
    });

    it('should count by severity', () => {
      const initialHistory = alertsManager.alertHistory.length;
      alertsManager.addToHistory({ name: 'C1', severity: 'critical', message: '' });
      alertsManager.addToHistory({ name: 'C2', severity: 'critical', message: '' });
      alertsManager.addToHistory({ name: 'H1', severity: 'high', message: '' });

      const stats = alertsManager.getStats();
      expect(stats.bySeverity.critical).toBeGreaterThanOrEqual(2);
      expect(stats.bySeverity.high).toBeGreaterThanOrEqual(1);
    });

    it('should track last 24h alerts', () => {
      const now = new Date();
      alertsManager.alertHistory = [
        { name: 'Recent', severity: 'info', message: '', receivedAt: now.toISOString() },
        { name: 'Old', severity: 'info', message: '', receivedAt: new Date(now - 48 * 3600000).toISOString() }
      ];

      const stats = alertsManager.getStats();
      expect(stats.last24h).toBe(1);
    });
  });
});
