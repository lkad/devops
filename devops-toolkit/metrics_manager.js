/**
 * Metrics Manager
 * Collects and exposes Prometheus-format metrics
 */

class MetricsManager {
  constructor() {
    // Counters: monotonically increasing values
    this.counters = new Map();

    // Gauges: can go up and down
    this.gauges = new Map();

    // Histograms: distribution of values
    this.histograms = new Map();

    // Labels for metrics
    this.defaultLabels = {
      service: 'devops-toolkit',
      version: '1.0.0'
    };
  }

  // Counter: monotonically increasing
  incCounter(name, labels = {}, value = 1) {
    const key = this.makeKey(name, labels);
    const current = this.counters.get(key) || 0;
    this.counters.set(key, current + value);
  }

  // Gauge: can go up and down
  setGauge(name, labels = {}, value) {
    const key = this.makeKey(name, labels);
    this.gauges.set(key, value);
  }

  incGauge(name, labels = {}, value = 1) {
    const key = this.makeKey(name, labels);
    const current = this.gauges.get(key) || 0;
    this.gauges.set(key, current + value);
  }

  decGauge(name, labels = {}, value = 1) {
    const key = this.makeKey(name, labels);
    const current = this.gauges.get(key) || 0;
    this.gauges.set(key, current - value);
  }

  // Histogram: observe a value
  observeHistogram(name, labels = {}, value) {
    const key = this.makeKey(name, labels);
    if (!this.histograms.has(key)) {
      this.histograms.set(key, []);
    }
    this.histograms.get(key).push(value);
  }

  // Record request latency
  recordLatency(endpoint, method, statusCode, durationMs) {
    const labels = { endpoint, method, status: statusCode };
    this.observeHistogram('http_request_duration_ms', labels, durationMs);
    this.incCounter('http_requests_total', labels);
  }

  // Record log event
  recordLog(level) {
    this.incCounter('logs_total', { level });
  }

  // Record device event
  recordDeviceEvent(eventType) {
    this.incCounter('device_events_total', { type: eventType });
  }

  // Record pipeline event
  recordPipelineEvent(eventType, pipelineId) {
    this.incCounter('pipeline_events_total', { type: eventType, pipeline: pipelineId });
  }

  // Record alert
  recordAlert(alertName, severity) {
    this.incCounter('alerts_total', { name: alertName, severity });
  }

  makeKey(name, labels) {
    const labelParts = Object.entries({ ...this.defaultLabels, ...labels })
      .map(([k, v]) => `${k}="${v}"`)
      .join(',');
    return `${name}{${labelParts}}`;
  }

  // Export in Prometheus format
  exportPrometheus() {
    const lines = [];

    // Header
    lines.push('# HELP devops_toolkit_info DevOps Toolkit information');
    lines.push('# TYPE devops_toolkit_info gauge');
    lines.push('devops_toolkit_info{service="devops-toolkit",version="1.0.0"} 1');

    // Counters
    for (const [key, value] of this.counters) {
      lines.push(`# TYPE ${this.extractName(key)} counter`);
      lines.push(`${key} ${value}`);
    }

    // Gauges
    for (const [key, value] of this.gauges) {
      lines.push(`# TYPE ${this.extractName(key)} gauge`);
      lines.push(`${key} ${value}`);
    }

    // Histograms (simplified - just show recent values stats)
    for (const [key, values] of this.histograms) {
      if (values.length === 0) continue;
      const sorted = [...values].sort((a, b) => a - b);
      const sum = values.reduce((a, b) => a + b, 0);
      const count = values.length;
      const min = sorted[0];
      const max = sorted[count - 1];
      const avg = sum / count;
      const p50 = sorted[Math.floor(count * 0.5)];
      const p95 = sorted[Math.floor(count * 0.95)];
      const p99 = sorted[Math.floor(count * 0.99)];

      const baseName = this.extractName(key);
      lines.push(`# TYPE ${baseName}_sum gauge`);
      lines.push(`${baseName}_sum${key.slice(key.indexOf('}'))} ${sum}`);
      lines.push(`# TYPE ${baseName}_count gauge`);
      lines.push(`${baseName}_count${key.slice(key.indexOf('}'))} ${count}`);
      lines.push(`# TYPE ${baseName}_min gauge`);
      lines.push(`${baseName}_min${key.slice(key.indexOf('}'))} ${min}`);
      lines.push(`# TYPE ${baseName}_max gauge`);
      lines.push(`${baseName}_max${key.slice(key.indexOf('}'))} ${max}`);
      lines.push(`# TYPE ${baseName}_avg gauge`);
      lines.push(`${baseName}_avg${key.slice(key.indexOf('}'))} ${avg.toFixed(2)}`);
      lines.push(`# TYPE ${baseName}_p50 gauge`);
      lines.push(`${baseName}_p50${key.slice(key.indexOf('}'))} ${p50}`);
      lines.push(`# TYPE ${baseName}_p95 gauge`);
      lines.push(`${baseName}_p95${key.slice(key.indexOf('}'))} ${p95}`);
      lines.push(`# TYPE ${baseName}_p99 gauge`);
      lines.push(`${baseName}_p99${key.slice(key.indexOf('}'))} ${p99}`);
    }

    return lines.join('\n');
  }

  extractName(key) {
    // Extract metric name from key (before the {)
    return key.replace(/\{.*/, '').replace(/^/,'devops_toolkit_');
  }

  // Get metrics as JSON for API
  getMetricsJSON() {
    const metrics = {
      counters: {},
      gauges: {},
      histograms: {},
      timestamp: new Date().toISOString()
    };

    for (const [key, value] of this.counters) {
      metrics.counters[key] = value;
    }

    for (const [key, value] of this.gauges) {
      metrics.gauges[key] = value;
    }

    for (const [key, values] of this.histograms) {
      if (values.length === 0) continue;
      const sorted = [...values].sort((a, b) => a - b);
      const sum = values.reduce((a, b) => a + b, 0);
      const count = values.length;
      metrics.histograms[key] = {
        count,
        sum,
        min: sorted[0],
        max: sorted[count - 1],
        avg: sum / count,
        p50: sorted[Math.floor(count * 0.5)],
        p95: sorted[Math.floor(count * 0.95)],
        p99: sorted[Math.floor(count * 0.99)]
      };
    }

    return metrics;
  }
}

// Singleton instance
const metricsManager = new MetricsManager();

module.exports = metricsManager;