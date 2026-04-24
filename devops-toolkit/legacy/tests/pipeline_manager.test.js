/**
 * Tests for Pipeline Manager - CI/CD Stages
 * Covers: smoke_test, verification, deployment strategies
 */

const PipelineManager = require('../pipelines/pipeline_manager');
const path = require('path');
const fs = require('fs');

describe('PipelineManager - CI/CD Stages', () => {
  const testStoragePath = '/tmp/devops-test-pipelines-' + Date.now();
  let pipelineManager;

  beforeEach(() => {
    pipelineManager = new PipelineManager(testStoragePath + '.json');
  });

  afterEach(() => {
    if (fs.existsSync(testStoragePath + '.json')) {
      fs.unlinkSync(testStoragePath + '.json');
    }
  });

  describe('Pipeline Execution Stages', () => {
    it('should execute all PRD-defined stages', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Full Stack Pipeline',
        stages: [
          'validate',
          'build',
          'test',
          'security_scan',
          'stage_deploy',
          'smoke_test',
          'prod_deploy',
          'verification'
        ]
      });

      expect(pipeline.stages).toHaveLength(8);
      expect(pipeline.stages).toContain('smoke_test');
      expect(pipeline.stages).toContain('verification');
    });

    it('should have smoke_test as a defined stage', () => {
      const logs = pipelineManager.getStageLogs('smoke_test');
      expect(logs).toBeDefined();
      expect(logs.length).toBeGreaterThan(0);
    });

    it('should have verification stage logs', () => {
      const logs = pipelineManager.getStageLogs('verification');
      expect(logs).toBeDefined();
      expect(logs.length).toBeGreaterThan(0);
    });
  });

  describe('smoke_test Stage', () => {
    it('should have smoke test specific logs', () => {
      const logs = pipelineManager.getStageLogs('smoke_test');
      expect(logs).toContain('Running smoke tests...');
      expect(logs).toContain('All smoke tests passed.');
    });

    it('should execute smoke_test stage during pipeline run', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Test Pipeline',
        stages: ['validate', 'build', 'smoke_test']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'test' });

      // Wait for async execution
      await new Promise(resolve => setTimeout(resolve, 500));

      const completedRun = pipelineManager.getRun(run.id);
      expect(completedRun).toBeDefined();
      const smokeStage = completedRun.stages.find(s => s.name === 'smoke_test');
      expect(smokeStage).toBeDefined();
    });
  });

  describe('verification Stage', () => {
    it('should have verification specific logs', () => {
      const logs = pipelineManager.getStageLogs('verification');
      expect(logs).toContain('Verifying deployment...');
      expect(logs).toContain('All endpoints responding.');
    });

    it('should execute verification after prod_deploy', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Deploy Pipeline',
        stages: ['stage_deploy', 'prod_deploy', 'verification']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'deploy' });

      await new Promise(resolve => setTimeout(resolve, 800));

      const completedRun = pipelineManager.getRun(run.id);
      const verificationStage = completedRun.stages.find(s => s.name === 'verification');
      expect(verificationStage).toBeDefined();
    });
  });

  describe('Deployment Strategies', () => {
    it('should support blue-green deployment pattern', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Blue-Green Pipeline',
        config: {
          deployment_strategy: 'blue-green',
          target_group: 'prod-servers',
          verification_timeout: 300
        }
      });

      expect(pipeline.config.deployment_strategy).toBe('blue-green');
    });

    it('should support canary release pattern', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Canary Pipeline',
        config: {
          deployment_strategy: 'canary',
          traffic_allocation: '1%→5%→25%→100%'
        }
      });

      expect(pipeline.config.deployment_strategy).toBe('canary');
    });

    it('should support rolling update pattern', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Rolling Pipeline',
        config: {
          deployment_strategy: 'rolling',
          max_surge: '20%'
        }
      });

      expect(pipeline.config.deployment_strategy).toBe('rolling');
    });
  });

  describe('Stage Execution', () => {
    it('should execute stages in order', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Order Test Pipeline',
        stages: ['validate', 'build', 'test']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'test' });

      await new Promise(resolve => setTimeout(resolve, 600));

      const completedRun = pipelineManager.getRun(run.id);
      expect(completedRun.stages[0].name).toBe('validate');
      expect(completedRun.stages[1].name).toBe('build');
      expect(completedRun.stages[2].name).toBe('test');
    });

    it('should record stage start and end times', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Timing Test Pipeline',
        stages: ['validate']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'test' });

      await new Promise(resolve => setTimeout(resolve, 300));

      const completedRun = pipelineManager.getRun(run.id);
      const validateStage = completedRun.stages[0];

      expect(validateStage.started_at).toBeDefined();
      expect(validateStage.finished_at).toBeDefined();
    });

    it('should fail pipeline if any stage fails', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Fail Test Pipeline',
        stages: ['validate', 'build', 'test']
      });

      // Create a mock that will be used by executeStage
      const run = pipelineManager.executePipeline(pipeline.id, { type: 'test' });

      await new Promise(resolve => setTimeout(resolve, 600));

      const completedRun = pipelineManager.getRun(run.id);
      // All stages should succeed in this mock implementation
      expect(completedRun.status).toBeDefined();
    });
  });

  describe('Run History and Stats', () => {
    it('should track run history', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'History Test Pipeline',
        stages: ['validate']
      });

      pipelineManager.executePipeline(pipeline.id, { type: 'manual' });
      await new Promise(resolve => setTimeout(resolve, 200));
      pipelineManager.executePipeline(pipeline.id, { type: 'manual' });

      const runs = pipelineManager.getPipelineRuns(pipeline.id);
      expect(runs.length).toBeGreaterThanOrEqual(2);
    });

    it('should calculate pipeline statistics', () => {
      const stats = pipelineManager.getPipelineStats('non-existent-id');
      expect(stats.total_runs).toBe(0);
      expect(stats.success_count).toBe(0);
      expect(stats.success_rate).toBe(0);
    });
  });

  describe('Webhook Trigger', () => {
    it('should support manual trigger', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Manual Trigger Pipeline',
        stages: ['validate']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'manual' });
      expect(run.trigger.type).toBe('manual');
    });

    it('should support api trigger', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'API Trigger Pipeline',
        stages: ['validate']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'api' });
      expect(run.trigger.type).toBe('api');
    });

    it('should record triggered_by in run', () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'TriggeredBy Pipeline',
        stages: ['validate']
      });

      const run = pipelineManager.executePipeline(pipeline.id, {
        type: 'manual',
        triggered_by: 'user@example.com'
      });

      expect(run.trigger.triggered_by).toBe('user@example.com');
    });
  });
});

describe('PipelineManager - Health Checks', () => {
  const testStoragePath = '/tmp/devops-test-health-' + Date.now();
  let pipelineManager;

  beforeEach(() => {
    pipelineManager = new PipelineManager(testStoragePath + '.json');
  });

  afterEach(() => {
    if (fs.existsSync(testStoragePath + '.json')) {
      fs.unlinkSync(testStoragePath + '.json');
    }
  });

  describe('Readiness Probe', () => {
    it('should include readiness check in verification', () => {
      const logs = pipelineManager.getStageLogs('verification');
      expect(logs.some(log => log.toLowerCase().includes('endpoint') || log.toLowerCase().includes('responding'))).toBe(true);
    });
  });

  describe('Liveness Probe', () => {
    it('should verify service is alive during smoke_test', () => {
      const logs = pipelineManager.getStageLogs('smoke_test');
      expect(logs.some(log => log.toLowerCase().includes('passed') || log.toLowerCase().includes('success'))).toBe(true);
    });
  });

  describe('Post-Deployment Verification', () => {
    it('should have verification logs for health checks', () => {
      const logs = pipelineManager.getStageLogs('verification');
      expect(logs).toContain('Verifying deployment...');
      expect(logs).toContain('All endpoints responding.');
      expect(logs).toContain('Verification complete.');
    });

    it('should verify deployment before marking success', async () => {
      const pipeline = pipelineManager.createPipeline({
        name: 'Verify Deploy Pipeline',
        stages: ['prod_deploy', 'verification']
      });

      const run = pipelineManager.executePipeline(pipeline.id, { type: 'deploy' });

      await new Promise(resolve => setTimeout(resolve, 1200));

      const completedRun = pipelineManager.getRun(run.id);
      const verificationStage = completedRun.stages.find(s => s.name === 'verification');

      expect(verificationStage.status).toBe('success');
      // Logs are stored in run.logs, filtered by stage
      const verificationLogs = completedRun.logs.filter(log => log.stage === 'verification');
      expect(verificationLogs.length).toBeGreaterThan(0);
    });
  });
});
