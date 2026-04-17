/**
 * Pipeline Manager
 * Manages CI/CD pipelines based on DESIGN.md Section 1
 */

const fs = require('fs');
const path = require('path');
const { v4: uuidv4 } = require('uuid');

class PipelineManager {
  constructor(storagePath) {
    this.storagePath = storagePath || path.join(__dirname, '../config/pipelines.json');
    this.pipelines = new Map();
    this.runs = new Map();
    this.load();
  }

  load() {
    try {
      if (fs.existsSync(this.storagePath)) {
        const data = JSON.parse(fs.readFileSync(this.storagePath, 'utf8'));
        this.pipelines = new Map(data.pipelines || []);
        this.runs = new Map(data.runs || []);
      }
    } catch (e) {
      console.error('Failed to load pipelines:', e.message);
    }
  }

  save() {
    try {
      const data = {
        pipelines: Array.from(this.pipelines.entries()),
        runs: Array.from(this.runs.entries())
      };
      fs.writeFileSync(this.storagePath, JSON.stringify(data, null, 2));
    } catch (e) {
      console.error('Failed to save pipelines:', e.message);
    }
  }

  // Pipeline CRUD
  createPipeline(config) {
    const id = config.id || uuidv4();
    const pipeline = {
      id,
      name: config.name,
      description: config.description || '',
      repository: config.repository || '',
      branch: config.branch || 'main',
      stages: config.stages || ['validate', 'build', 'test', 'deploy'],
      config: config.config || {},
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    };

    this.pipelines.set(id, pipeline);
    this.save();
    return pipeline;
  }

  getPipeline(id) {
    return this.pipelines.get(id);
  }

  getAllPipelines() {
    return Array.from(this.pipelines.values());
  }

  updatePipeline(id, updates) {
    const pipeline = this.pipelines.get(id);
    if (!pipeline) return null;

    const updated = {
      ...pipeline,
      ...updates,
      id,
      updated_at: new Date().toISOString()
    };

    this.pipelines.set(id, updated);
    this.save();
    return updated;
  }

  deletePipeline(id) {
    const result = this.pipelines.delete(id);
    if (result) this.save();
    return result;
  }

  // Pipeline Run execution
  executePipeline(pipelineId, trigger = {}) {
    const pipeline = this.pipelines.get(pipelineId);
    if (!pipeline) return null;

    const runId = uuidv4();
    const run = {
      id: runId,
      pipeline_id: pipelineId,
      pipeline_name: pipeline.name,
      status: 'pending',
      stages: this.initializeStages(pipeline.stages),
      trigger,
      started_at: new Date().toISOString(),
      finished_at: null,
      logs: []
    };

    this.runs.set(runId, run);
    this.save();

    // Execute stages asynchronously
    this.executeStagesAsync(runId);

    return run;
  }

  initializeStages(stageNames) {
    return stageNames.map(name => ({
      name,
      status: 'pending',
      started_at: null,
      finished_at: null,
      logs: [],
      error: null
    }));
  }

  async executeStagesAsync(runId) {
    const run = this.runs.get(runId);
    if (!run) return;

    for (let i = 0; i < run.stages.length; i++) {
      const stage = run.stages[i];
      stage.status = 'running';
      stage.started_at = new Date().toISOString();
      this.runs.set(runId, run);
      this.save();

      try {
        await this.executeStage(runId, stage);
        stage.status = 'success';
        stage.finished_at = new Date().toISOString();
      } catch (error) {
        stage.status = 'failed';
        stage.error = error.message;
        stage.finished_at = new Date().toISOString();
        run.status = 'failed';
        this.runs.set(runId, run);
        this.save();
        return;
      }

      this.runs.set(runId, run);
      this.save();
    }

    run.status = 'success';
    run.finished_at = new Date().toISOString();
    this.runs.set(runId, run);
    this.save();
  }

  async executeStage(runId, stage) {
    // Simulate stage execution with logs
    const logs = this.getStageLogs(stage.name);

    for (const log of logs) {
      const run = this.runs.get(runId);
      if (!run) return;
      run.logs.push({
        timestamp: new Date().toISOString(),
        stage: stage.name,
        message: log
      });
      this.runs.set(runId, run);
      this.save();
      await this.sleep(100);
    }

    // Simulate stage-specific work
    await this.sleep(200);
  }

  getStageLogs(stageName) {
    const logs = {
      validate: [
        'Checking pipeline configuration...',
        'Validating repository access...',
        'Repository access verified.',
        'Configuration validated successfully.'
      ],
      build: [
        'Pulling dependencies...',
        'Building application...',
        'Build completed successfully.',
        'Image size: 125MB'
      ],
      test: [
        'Running unit tests...',
        'Running integration tests...',
        'All tests passed (42/42).',
        'Code coverage: 85%'
      ],
      deploy: [
        'Preparing deployment...',
        'Deploying to target environment...',
        'Health checks passed.',
        'Deployment successful.'
      ],
      security_scan: [
        'Scanning for vulnerabilities...',
        'No critical vulnerabilities found.',
        'Security scan completed.'
      ],
      smoke_test: [
        'Running smoke tests...',
        'All smoke tests passed.',
        'Smoke tests completed.'
      ],
      prod_deploy: [
        'Starting production deployment...',
        'Rolling update in progress...',
        'New version deployed.',
        'Production deployment complete.'
      ],
      verification: [
        'Verifying deployment...',
        'All endpoints responding.',
        'Verification complete.'
      ]
    };

    return logs[stageName] || [`Executing ${stageName}...`, `${stageName} completed.`];
  }

  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  // Run operations
  getRun(runId) {
    return this.runs.get(runId);
  }

  getPipelineRuns(pipelineId) {
    return Array.from(this.runs.values())
      .filter(run => run.pipeline_id === pipelineId)
      .sort((a, b) => new Date(b.started_at) - new Date(a.started_at));
  }

  getAllRuns() {
    return Array.from(this.runs.values())
      .sort((a, b) => new Date(b.started_at) - new Date(a.started_at));
  }

  getRecentRuns(limit = 10) {
    return this.getAllRuns().slice(0, limit);
  }

  // Get pipeline statistics
  getPipelineStats(pipelineId) {
    const runs = this.getPipelineRuns(pipelineId);
    const total = runs.length;
    const success = runs.filter(r => r.status === 'success').length;
    const failed = runs.filter(r => r.status === 'failed').length;
    const avgDuration = total > 0
      ? runs.reduce((sum, r) => {
          if (r.finished_at && r.started_at) {
            return sum + (new Date(r.finished_at) - new Date(r.started_at));
          }
          return sum;
        }, 0) / total
      : 0;

    return {
      total_runs: total,
      success_count: success,
      failed_count: failed,
      success_rate: total > 0 ? (success / total * 100).toFixed(1) : 0,
      avg_duration_ms: Math.round(avgDuration)
    };
  }

  // Delete old artifacts (cleanup)
  deleteRun(runId) {
    const result = this.runs.delete(runId);
    if (result) this.save();
    return result;
  }

  // Cancel running pipeline
  cancelRun(runId) {
    const run = this.runs.get(runId);
    if (!run) return null;

    if (run.status === 'running') {
      const currentStage = run.stages.find(s => s.status === 'running');
      if (currentStage) {
        currentStage.status = 'cancelled';
        currentStage.finished_at = new Date().toISOString();
      }
      run.status = 'cancelled';
      run.finished_at = new Date().toISOString();
      this.runs.set(runId, run);
      this.save();
    }

    return run;
  }
}

module.exports = PipelineManager;
