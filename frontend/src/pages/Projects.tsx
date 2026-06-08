// Projects page — two-pane layout: 3-level project tree on the left,
// selected project detail (metadata + members) on the right.
// Tree is built client-side from the flat GET /api/v1/projects list.
import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Badge } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { FormField } from '../components/common/FormField';
import { EmptyState } from '../components/common/EmptyState';
import { useToast } from '../components/common/Toast';
import { useApi } from '../hooks/useApi';
import { apiPost, ListResponse } from '../api/client';

interface Project {
  id: number;
  name: string;
  code: string;
  type_id: number;
  parent_id: number | null;
  weight: number;
  created_at?: string;
  updated_at?: string;
}

interface ProjectMember {
  user_id: number;
  username: string;
  role: string;
  joined_at?: string;
}

interface ProjectType {
  id: number;
  name: string;
  level: number;
}

interface TreeNode {
  project: Project;
  children: TreeNode[];
}

function buildTree(projects: Project[]): TreeNode[] {
  const byId = new Map<number, TreeNode>();
  projects.forEach((p) => byId.set(p.id, { project: p, children: [] }));
  const roots: TreeNode[] = [];
  byId.forEach((node) => {
    const pid = node.project.parent_id;
    if (pid !== null && pid !== undefined && byId.has(pid)) {
      byId.get(pid)!.children.push(node);
    } else {
      roots.push(node);
    }
  });
  // Stable order: sort siblings by id ascending.
  const sortRec = (nodes: TreeNode[]) => {
    nodes.sort((a, b) => a.project.id - b.project.id);
    nodes.forEach((n) => sortRec(n.children));
  };
  sortRec(roots);
  return roots;
}

function flattenTree(
  nodes: TreeNode[],
  expanded: Set<number>,
  depth = 0,
  typeNameById: Map<number, string>,
): Array<{ node: TreeNode; depth: number; level: number }> {
  const out: Array<{ node: TreeNode; depth: number; level: number }> = [];
  for (const n of nodes) {
    const level = depth;
    out.push({ node: n, depth, level });
    if (expanded.has(n.project.id) && n.children.length > 0) {
      out.push(...flattenTree(n.children, expanded, depth + 1, typeNameById));
    }
  }
  return out;
}

export function Projects() {
  const toast = useToast();
  const { data, loading, error, reload } = useApi<ListResponse<Project>>('projects');
  const { data: typesData } = useApi<ListResponse<ProjectType>>('project-types');

  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState({ name: '', code: '', type_id: '', parent_id: '' });
  const [submitting, setSubmitting] = useState(false);

  const typeNameById = useMemo(() => {
    const m = new Map<number, string>();
    (typesData?.data ?? []).forEach((t) => m.set(t.id, t.name));
    return m;
  }, [typesData]);

  const tree = useMemo(() => buildTree(data?.data ?? []), [data]);

  // Auto-expand roots and auto-select the first project.
  useEffect(() => {
    if (!data) return;
    setExpanded((cur) => {
      if (cur.size > 0) return cur;
      const next = new Set<number>();
      tree.forEach((n) => next.add(n.project.id));
      return next;
    });
    if (selectedId === null && data.data.length > 0) {
      setSelectedId(data.data[0].id);
    }
  }, [data, tree, selectedId]);

  const flat = useMemo(
    () => flattenTree(tree, expanded, 0, typeNameById),
    [tree, expanded, typeNameById],
  );

  const selected = useMemo(() => {
    if (selectedId === null) return null;
    return (data?.data ?? []).find((p) => p.id === selectedId) ?? null;
  }, [selectedId, data]);

  const parent = useMemo(() => {
    if (!selected || selected.parent_id === null || selected.parent_id === undefined) return null;
    return (data?.data ?? []).find((p) => p.id === selected.parent_id) ?? null;
  }, [selected, data]);

  const membersPath = selected ? `projects/${selected.id}/members` : null;
  const { data: membersData, loading: membersLoading, reload: reloadMembers } = useApi<ListResponse<ProjectMember>>(membersPath);

  const toggleExpand = (id: number) => {
    setExpanded((cur) => {
      const next = new Set(cur);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const openCreate = () => {
    setForm({ name: '', code: '', type_id: '', parent_id: '' });
    setCreateOpen(true);
  };

  const submitCreate = async () => {
    if (!form.name.trim() || !form.code.trim() || !form.type_id) {
      toast.push('Name, Code, and Type are required', 'error');
      return;
    }
    setSubmitting(true);
    try {
      const body: Record<string, unknown> = {
        name: form.name.trim(),
        code: form.code.trim(),
        type_id: Number(form.type_id),
      };
      if (form.parent_id.trim()) body.parent_id = Number(form.parent_id);
      await apiPost('projects', body);
      toast.push('Project created', 'success');
      setCreateOpen(false);
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to create project', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const memberColumns: Column<ProjectMember>[] = [
    { key: 'user_id', header: 'User ID', render: (r) => <span className="mono">{r.user_id}</span> },
    { key: 'username', header: 'Username', render: (r) => r.username },
    { key: 'role', header: 'Role', render: (r) => <Badge tone="neutral">{r.role}</Badge> },
    { key: 'joined_at', header: 'Joined', render: (r) => (r.joined_at ? new Date(r.joined_at).toLocaleString() : '—') },
  ];

  return (
    <div>
      <PageHeader
        title="Projects"
        subtitle="Business Line → System → Project hierarchy"
        actions={<Button variant="primary" onClick={openCreate}>+ New Project</Button>}
      />

      {error && (
        <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-4)' }}>{error}</div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: '320px 1fr', gap: 'var(--sp-4)', alignItems: 'start' }}>
        {/* Left pane: tree */}
        <div
          style={{
            background: 'var(--color-surface)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-md)',
            padding: 'var(--sp-3)',
            minHeight: 480,
            maxHeight: 'calc(100vh - 220px)',
            overflow: 'auto',
          }}
        >
          {loading && <div style={{ color: 'var(--color-text-muted)', padding: 'var(--sp-3)' }}>Loading…</div>}
          {!loading && (data?.data?.length ?? 0) === 0 && (
            <div style={{ padding: 'var(--sp-3)', color: 'var(--color-text-muted)' }}>No projects yet</div>
          )}
          {flat.map(({ node, depth }) => {
            const p = node.project;
            const isExpanded = expanded.has(p.id);
            const hasChildren = node.children.length > 0;
            const isSelected = p.id === selectedId;
            return (
              <div
                key={p.id}
                onClick={() => setSelectedId(p.id)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 'var(--sp-2)',
                  padding: '6px 8px',
                  paddingLeft: 8 + depth * 16,
                  borderRadius: 'var(--radius-sm)',
                  cursor: 'pointer',
                  background: isSelected ? 'var(--color-primary-muted)' : 'transparent',
                  color: isSelected ? 'var(--color-primary)' : 'var(--color-text)',
                  fontSize: 'var(--fs-small)',
                }}
              >
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    if (hasChildren) toggleExpand(p.id);
                  }}
                  style={{
                    width: 16,
                    display: 'inline-block',
                    textAlign: 'center',
                    color: 'var(--color-text-muted)',
                    cursor: hasChildren ? 'pointer' : 'default',
                    userSelect: 'none',
                  }}
                >
                  {hasChildren ? (isExpanded ? '▾' : '▸') : '·'}
                </span>
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {p.name}
                </span>
                <span className="mono" style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)' }}>
                  {p.code}
                </span>
              </div>
            );
          })}
        </div>

        {/* Right pane: detail */}
        <div style={{ minHeight: 480 }}>
          {!selected && (
            <EmptyState title="Select a project" hint="Pick a node from the tree on the left to see its details." />
          )}
          {selected && (
            <div
              style={{
                background: 'var(--color-surface)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-md)',
                padding: 'var(--sp-5)',
                marginBottom: 'var(--sp-4)',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 'var(--sp-4)' }}>
                <div>
                  <h2 style={{ marginBottom: 4 }}>{selected.name}</h2>
                  <div className="mono" style={{ color: 'var(--color-text-muted)' }}>{selected.code}</div>
                </div>
                <Badge tone="primary">L{(parent ? 2 : 0) + 1}</Badge>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 'var(--sp-4)' }}>
                <Field label="ID" value={<span className="mono">{selected.id}</span>} />
                <Field label="Name" value={selected.name} />
                <Field label="Code" value={<span className="mono">{selected.code}</span>} />
                <Field label="Type" value={typeNameById.get(selected.type_id) ?? `#${selected.type_id}`} />
                <Field label="Parent" value={parent ? `${parent.name} (#${parent.id})` : '— (root)'} />
                <Field label="Weight" value={<span className="mono">{selected.weight}</span>} />
              </div>
            </div>
          )}

          {selected && (
            <div>
              <h3 style={{ marginBottom: 'var(--sp-3)' }}>Members</h3>
              <DataTable<ProjectMember>
                rows={membersData?.data ?? null}
                loading={membersLoading}
                columns={memberColumns}
                rowKey={(r) => String(r.user_id)}
                empty={{ title: 'No members', hint: 'This project has no members yet.' }}
              />
              <div style={{ marginTop: 'var(--sp-3)', display: 'flex', justifyContent: 'flex-end' }}>
                <Button variant="ghost" onClick={reloadMembers}>Refresh</Button>
              </div>
            </div>
          )}
        </div>
      </div>

      <Modal
        open={createOpen}
        onClose={() => (submitting ? null : setCreateOpen(false))}
        title="New Project"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateOpen(false)} disabled={submitting}>Cancel</Button>
            <Button variant="primary" onClick={submitCreate} disabled={submitting}>
              {submitting ? 'Creating…' : 'Create'}
            </Button>
          </>
        }
      >
        <FormField label="Name">
          {(s) => (
            <input
              style={s}
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="e.g. Payments Platform"
            />
          )}
        </FormField>
        <FormField label="Code" hint="Short uppercase identifier">
          {(s) => (
            <input
              style={s}
              className="mono"
              value={form.code}
              onChange={(e) => setForm({ ...form, code: e.target.value })}
              placeholder="e.g. PAY"
            />
          )}
        </FormField>
        <FormField label="Type ID" hint="Numeric type id from /project-types">
          {(s) => (
            <input
              style={s}
              className="mono"
              value={form.type_id}
              onChange={(e) => setForm({ ...form, type_id: e.target.value })}
              placeholder="1"
            />
          )}
        </FormField>
        <FormField label="Parent ID" hint="Optional. Leave blank for a top-level project.">
          {(s) => (
            <input
              style={s}
              className="mono"
              value={form.parent_id}
              onChange={(e) => setForm({ ...form, parent_id: e.target.value })}
              placeholder="e.g. 2"
            />
          )}
        </FormField>
      </Modal>
    </div>
  );
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 4 }}>
        {label}
      </div>
      <div style={{ fontSize: 'var(--fs-small)' }}>{value}</div>
    </div>
  );
}
