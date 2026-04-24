/**
 * Tests for Metrics Manager
 * Covers: Prometheus metrics collection and export
 */

const MetricsManager = require('../metrics_manager');

describe('MetricsManager', () => {
  let metrics;

  beforeEach(() => {
    jest.resetModules();
    metrics = require('../metrics_manager');
  });

  describe('Counter', () => {
    it('should increment counter', () => {
      metrics.incCounter('test_counter', {}, 1);
      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_test_counter');
    });

    it('should increment counter with labels', () => {
      metrics.incCounter('test_counter', { method: 'GET' }, 1);
      metrics.incCounter('test_counter', { method: 'POST' }, 1);

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_test_counter');
    });

    it('should increment by custom value', () => {
      metrics.incCounter('test_counter', {}, 5);
      const exported = metrics.exportPrometheus();
      expect(exported).toContain('5');
    });
  });

  describe('Gauge', () => {
    it('should set gauge value', () => {
      metrics.setGauge('test_gauge', { env: 'test' }, 100);
      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_test_gauge');
    });

    it('should increment gauge', () => {
      metrics.setGauge('test_gauge', {}, 10);
      metrics.incGauge('test_gauge', {}, 5);

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_test_gauge');
    });

    it('should decrement gauge', () => {
      metrics.setGauge('test_gauge', {}, 10);
      metrics.decGauge('test_gauge', {}, 3);

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_test_gauge');
    });
  });

  describe('Histogram', () => {
    it('should observe histogram value', () => {
      metrics.observeHistogram('test_histogram', { path: '/api' }, 0.5);
      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_test_histogram');
    });

    it('should record latency', () => {
      metrics.recordLatency('/api/test', 'GET', 200, 150);

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_http_request_duration_ms');
    });
  });

  describe('recordLatency', () => {
    it('should record HTTP latency metrics', () => {
      metrics.recordLatency('/api/users', 'POST', 201, 250);

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('method="POST"');
      expect(exported).toContain('status="201"');
    });

    it('should record different status codes', () => {
      metrics.recordLatency('/api/error', 'GET', 500, 100);
      metrics.recordLatency('/api/redirect', 'GET', 302, 50);

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('status="500"');
    });
  });

  describe('recordLog', () => {
    it('should record log events', () => {
      metrics.recordLog('info');
      metrics.recordLog('warn');
      metrics.recordLog('error');

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_logs_total');
    });
  });

  describe('recordDeviceEvent', () => {
    it('should record device events', () => {
      metrics.recordDeviceEvent('registered');
      metrics.recordDeviceEvent('state_changed');

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_device_events_total');
    });
  });

  describe('recordPipelineEvent', () => {
    it('should record pipeline events', () => {
      metrics.recordPipelineEvent('started', 'pipeline-1');
      metrics.recordPipelineEvent('completed', 'pipeline-1');

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_pipeline_events_total');
    });
  });

  describe('recordAlert', () => {
    it('should record alerts', () => {
      metrics.recordAlert('high_cpu', 'warning');
      metrics.recordAlert('disk_full', 'critical');

      const exported = metrics.exportPrometheus();
      expect(exported).toContain('devops_toolkit_alerts_total');
    });
  });

  describe('exportPrometheus', () => {
    it('should export in Prometheus format', () => {
      metrics.incCounter('export_test', {}, 1);

      const output = metrics.exportPrometheus();
      expect(typeof output).toBe('string');
      expect(output.length).toBeGreaterThan(0);
    });

    it('should include TYPE comments', () => {
      metrics.setGauge('typed_gauge', {}, 42);

      const output = metrics.exportPrometheus();
      expect(output).toContain('# TYPE');
    });

    it('should include HELP comments', () => {
      metrics.setGauge('helped_gauge', {}, 10);

      const output = metrics.exportPrometheus();
      expect(output).toContain('# HELP');
    });
  });

  describe('getMetricsJSON', () => {
    it('should export as JSON', () => {
      metrics.incCounter('json_test', {}, 1);

      const json = metrics.getMetricsJSON();
      expect(json).toBeDefined();
      expect(typeof json).toBe('object');
    });

    it('should include counter data', () => {
      metrics.incCounter('json_counter', { label: 'value' }, 5);

      const json = metrics.getMetricsJSON();
      expect(json.counters).toBeDefined();
    });
  });

  describe('makeKey', () => {
    it('should create key from name and labels', () => {
      const key = metrics.makeKey('test_metric', { method: 'GET' });
      expect(key).toContain('test_metric');
    });

    it('should handle empty labels', () => {
      const key = metrics.makeKey('test_metric', {});
      expect(key).toBeDefined();
    });
  });

  describe('extractName', () => {
    it('should extract name from metric key and add prefix', () => {
      const name = metrics.extractName('test_metric{method="GET"}');
      expect(name).toBe('devops_toolkit_test_metric');
    });

    it('should handle labels with special characters', () => {
      const name = metrics.extractName('metric{label="value with spaces"}');
      expect(name).toBe('devops_toolkit_metric');
    });
  });
});
