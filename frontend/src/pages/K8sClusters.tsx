// K8sClusters — K8s cluster list page.
// Toolbar with type filter and "+ New Cluster" button.
// Body: card grid (auto-fill, min 280px) of ClusterCard.
// Each card shows name (mono), type badge, status, "Probe" and "View" actions.
// View opens a modal with 3 tabs (Pods/Deployments/Services).

import { useState, useMemo } from 'react';
import { useApi } from '../hooks/useApi';
import { apiPost } from '../api/client';
import { Badge, toneForEnv } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { PageHeader } from '../components/common/PageHeader';
import { EmptyState } from '../components/common/EmptyState';
import { FormField } from '../components/common/FormField';
import { useToast } from '../components/common/Toast';
import { DataTable, Column } from '../components/common/DataTable';
import { PodLogsModal } from '../components/K8s/PodLogsModal';
import { PodExecModal } from '../components/K8s/PodExecModal';

type ClusterType = 'k3d' | 'kind' | 'standard';
type ClusterStatus = 'connected' | 'disconnected' | 'unknown';

interface K8sCluster {
  id: string;
  name: string;
  type: ClusterType;
  status: ClusterStatus;
  environment?: string;
  description?: string;
  api_endpoint?: string;
  created_at?: string;
}

type FilterType = 'all' | ClusterType;

function statusTone(s: ClusterStatus) {
  if (s === 'connected') return 'success' as const;
  if (s === 'disconnected') return 'error' as const;
  return 'neutral' as const;
}

function typeTone(t: ClusterType) {
  // Use env tones by mapping type → env slot; here we keep them
  // distinguishable but consistent: k3d=dev, kind=test, standard=prod.
  if (t === 'k3d') return 'info' as const;
  if (t === 'kind') return 'warning' as const;
  return 'error' as const;
}

export function K8sClusters() {
  const [typeFilter, setTypeFilter] = useState<FilterType>('all');
  const [viewing, setViewing] = useState<K8sCluster | null>(null);
  const [creating, setCreating] = useState(false);
  const [probingId, setProbingId] = useState<string | null>(null);
  const { push: toast } = useToast();

  const query = useMemo(
    () => (typeFilter === 'all' ? undefined : { type: typeFilter }),
    [typeFilter],
  );
  const { data, loading, error, reload } = useApi<{ data: K8sCluster[] }>(
    'k8s/clusters',
    query,
  );

  const clusters: K8sCluster[] = data?.data ?? [];

  async function handleProbe(c: K8sCluster) {
    setProbingId(c.id);
    try {
      await apiPost(`k8s/clusters/${c.id}/probe`);
      toast(`Probe triggered for ${c.name}`, 'success');
      reload();
    } catch (e: any) {
      toast(`Probe failed: ${e?.message ?? 'unknown error'}`, 'error');
    } finally {
      setProbingId(null);
    }
  }

  return (
    <div>
      <PageHeader
        title="K8s Clusters"
        subtitle="Manage registered Kubernetes clusters"
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            + New Cluster
          </Button>
        }
      />

      {/* Toolbar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--sp-2)',
          marginBottom: 'var(--sp-4)',
        }}
      >
        <span
          style={{
            fontSize: 'var(--fs-caption)',
            color: 'var(--color-text-secondary)',
            textTransform: 'uppercase',
            letterSpacing: 0.5,
            fontWeight: 600,
          }}
        >
          Type
        </span>
        {(['all', 'k3d', 'kind', 'standard'] as FilterType[]).map((t) => (
          <button
            key={t}
            onClick={() => setTypeFilter(t)}
            style={{
              background:
                typeFilter === t
                  ? 'var(--color-primary-muted)'
                  : 'var(--color-surface-elevated)',
              color:
                typeFilter === t
                  ? 'var(--color-primary)'
                  : 'var(--color-text-secondary)',
              border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-sm)',
              padding: '4px 10px',
              fontSize: 'var(--fs-small)',
              cursor: 'pointer',
              fontWeight: typeFilter === t ? 600 : 400,
              textTransform: 'capitalize',
            }}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Body */}
      {error && (
        <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-4)' }}>
          {error}
        </div>
      )}
      {loading ? (
        <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>
      ) : clusters.length === 0 ? (
        <EmptyState
          title="No clusters yet"
          hint="Register your first Kubernetes cluster to get started."
          action={
            <Button variant="primary" onClick={() => setCreating(true)}>
              + New Cluster
            </Button>
          }
        />
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
            gap: 'var(--sp-4)',
          }}
        >
          {clusters.map((c) => (
            <ClusterCard
              key={c.id}
              cluster={c}
              probing={probingId === c.id}
              onProbe={() => handleProbe(c)}
              onView={() => setViewing(c)}
            />
          ))}
        </div>
      )}

      {viewing && (
        <ClusterViewModal cluster={viewing} onClose={() => setViewing(null)} />
      )}

      {creating && (
        <NewClusterModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            toast('Cluster registered', 'success');
            reload();
          }}
        />
      )}
    </div>
  );
}

function ClusterCard({
  cluster,
  probing,
  onProbe,
  onView,
}: {
  cluster: K8sCluster;
  probing: boolean;
  onProbe: () => void;
  onView: () => void;
}) {
  return (
    <div
      style={{
        background: 'var(--color-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-md)',
        padding: 'var(--sp-4)',
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--sp-3)',
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          gap: 'var(--sp-2)',
        }}
      >
        <div className="mono" style={{ fontSize: 'var(--fs-mono)', wordBreak: 'break-all' }}>
          {cluster.name}
        </div>
        <Badge tone={typeTone(cluster.type)}>{cluster.type}</Badge>
      </div>

      {cluster.environment && (
        <div>
          <Badge tone={toneForEnv(cluster.environment)}>{cluster.environment}</Badge>
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-2)' }}>
        <span
          style={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            background:
              cluster.status === 'connected'
                ? 'var(--color-success)'
                : cluster.status === 'disconnected'
                ? 'var(--color-error)'
                : 'var(--color-text-muted)',
            display: 'inline-block',
          }}
        />
        <span style={{ fontSize: 'var(--fs-small)', color: 'var(--color-text-secondary)' }}>
          <Badge tone={statusTone(cluster.status)}>{cluster.status}</Badge>
        </span>
      </div>

      {cluster.description && (
        <div
          style={{
            fontSize: 'var(--fs-small)',
            color: 'var(--color-text-secondary)',
            minHeight: 18,
          }}
        >
          {cluster.description}
        </div>
      )}

      <div
        style={{
          display: 'flex',
          gap: 'var(--sp-2)',
          borderTop: '1px solid var(--color-border-subtle)',
          paddingTop: 'var(--sp-3)',
          marginTop: 'auto',
        }}
      >
        <Button
          variant="secondary"
          onClick={onProbe}
          disabled={probing}
        >
          {probing ? 'Probing…' : 'Probe'}
        </Button>
        <Button variant="primary" onClick={onView}>
          View
        </Button>
      </div>
    </div>
  );
}

type Tab = 'pods' | 'deployments' | 'services';

function ClusterViewModal({ cluster, onClose }: { cluster: K8sCluster; onClose: () => void }) {
  const [tab, setTab] = useState<Tab>('pods');
  const [namespace, setNamespace] = useState('');

  const nsQuery = useMemo(
    () => (namespace ? { namespace } : undefined),
    [namespace],
  );

  return (
    <Modal
      open
      onClose={onClose}
      title={`Cluster: ${cluster.name}`}
      width={760}
    >
      <div
        style={{
          display: 'flex',
          gap: 'var(--sp-2)',
          borderBottom: '1px solid var(--color-border)',
          marginBottom: 'var(--sp-4)',
        }}
      >
        {(['pods', 'deployments', 'services'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{
              background: 'transparent',
              border: 'none',
              borderBottom:
                tab === t ? '2px solid var(--color-primary)' : '2px solid transparent',
              color:
                tab === t ? 'var(--color-primary)' : 'var(--color-text-secondary)',
              padding: 'var(--sp-2) var(--sp-3)',
              fontSize: 'var(--fs-small)',
              fontWeight: tab === t ? 600 : 400,
              cursor: 'pointer',
              textTransform: 'capitalize',
            }}
          >
            {t}
          </button>
        ))}
        <div style={{ marginLeft: 'auto', alignSelf: 'center' }}>
          <input
            placeholder="namespace (optional)"
            value={namespace}
            onChange={(e) => setNamespace(e.target.value)}
            style={{
              background: 'var(--color-bg)',
              color: 'var(--color-text)',
              border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-sm)',
              padding: '4px 8px',
              fontSize: 'var(--fs-caption)',
            }}
          />
        </div>
      </div>

      <ResourceTab kind={tab} clusterId={cluster.id} query={nsQuery} />
    </Modal>
  );
}

interface Resource {
  name: string;
  namespace?: string;
  status?: string;
  [k: string]: any;
}

function ResourceTab({
  kind,
  clusterId,
  query,
}: {
  kind: Tab;
  clusterId: string;
  query: Record<string, any> | undefined;
}) {
  const path =
    kind === 'pods'
      ? `k8s/clusters/${clusterId}/pods`
      : kind === 'deployments'
      ? `k8s/clusters/${clusterId}/deployments`
      : `k8s/clusters/${clusterId}/services`;

  const { data, loading, error } = useApi<{ data: Resource[] }>(path, query);
  const rows = data?.data ?? [];

  // Per-pod drill-in state — only meaningful when kind === 'pods'.
  // Local to the tab so re-mounting on tab change resets cleanly.
  const [logsPod, setLogsPod] = useState<Resource | null>(null);
  const [execPod, setExecPod] = useState<Resource | null>(null);

  if (loading) return <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>;
  if (error) return <div style={{ color: 'var(--color-error)' }}>{error}</div>;
  if (rows.length === 0) {
    return (
      <EmptyState
        title={`No ${kind} found`}
        hint={query?.namespace ? `Namespace: ${query.namespace}` : 'No namespace filter applied.'}
      />
    );
  }

  const columns: Column<Resource>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (r) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-2)', flexWrap: 'wrap' }}>
          <span className="mono">{r.name}</span>
          {kind === 'pods' && (
            <>
              <Button
                variant="ghost"
                onClick={() => setLogsPod(r)}
              >
                <span data-testid={`pod-logs-button-${r.name}`}>Logs</span>
              </Button>
              <Button
                variant="ghost"
                onClick={() => setExecPod(r)}
              >
                <span data-testid={`pod-shell-button-${r.name}`}>Shell</span>
              </Button>
            </>
          )}
        </div>
      ),
    },
    {
      key: 'namespace',
      header: 'Namespace',
      render: (r) => r.namespace ?? '—',
    },
    {
      key: 'status',
      header: 'Status',
      render: (r) => r.status ?? '—',
    },
  ];

  return (
    <>
      <DataTable rows={rows} columns={columns} rowKey={(r) => `${kind}-${r.name}-${r.namespace ?? ''}`} empty={{ title: '—' }} />
      {kind === 'pods' && logsPod && (
        <PodLogsModal
          open
          onClose={() => setLogsPod(null)}
          clusterId={clusterId}
          namespace={logsPod.namespace ?? (query?.namespace as string) ?? 'default'}
          pod={logsPod.name}
        />
      )}
      {kind === 'pods' && execPod && (
        <PodExecModal
          open
          onClose={() => setExecPod(null)}
          clusterId={clusterId}
          namespace={execPod.namespace ?? (query?.namespace as string) ?? 'default'}
          pod={execPod.name}
        />
      )}
    </>
  );
}

function NewClusterModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState('');
  const [type, setType] = useState<ClusterType>('standard');
  const [endpoint, setEndpoint] = useState('');
  const [environment, setEnvironment] = useState('dev');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const { push: toast } = useToast();

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      await apiPost('k8s/clusters', {
        name,
        type,
        api_endpoint: endpoint,
        environment,
        description,
      });
      onCreated();
    } catch (e: any) {
      const m = e?.message ?? 'create failed';
      setErr(m);
      toast(m, 'error');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="New Cluster"
      width={520}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={submitting || !name}>
            {submitting ? 'Creating…' : 'Create'}
          </Button>
        </>
      }
    >
      {err && (
        <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>{err}</div>
      )}
      <FormField label="Name">
        {(s) => <input style={s} value={name} onChange={(e) => setName(e.target.value)} placeholder="prod-us-east-1" />}
      </FormField>
      <FormField label="Type">
        {(s) => (
          <select style={s} value={type} onChange={(e) => setType(e.target.value as ClusterType)}>
            <option value="k3d">k3d</option>
            <option value="kind">kind</option>
            <option value="standard">standard</option>
          </select>
        )}
      </FormField>
      <FormField label="API Endpoint">
        {(s) => <input style={s} value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="https://k8s.example.com:6443" />}
      </FormField>
      <FormField label="Environment">
        {(s) => (
          <select style={s} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
            <option value="dev">dev</option>
            <option value="test">test</option>
            <option value="uat">uat</option>
            <option value="prod">prod</option>
          </select>
        )}
      </FormField>
      <FormField label="Description">
        {(s) => <input style={s} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" />}
      </FormField>
    </Modal>
  );
}
