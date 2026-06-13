// RunbookEditor — modal form to create a new runbook entry.
//
// POSTs to /api/v1/services/:serviceID/runbook. Tags are
// comma-separated and split into an array before submission.

import { useState } from 'react';
import { Modal } from '../common/Modal';
import { Button } from '../common/Button';
import { FormField } from '../common/FormField';
import { apiPost } from '../../api/client';

export interface RunbookEditorProps {
  open: boolean;
  onClose: () => void;
  serviceID: string;
  onSaved: () => void;
}

export function RunbookEditor({ open, onClose, serviceID, onSaved }: RunbookEditorProps) {
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [tags, setTags] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const submit = async () => {
    if (!title || !content) {
      setError('title and content are required');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await apiPost(`/api/v1/services/${serviceID}/runbook`, {
        title,
        content,
        tags: tags
          .split(',')
          .map((t) => t.trim())
          .filter(Boolean),
      });
      onSaved();
      onClose();
    } catch (e: any) {
      setError(e?.message || String(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Add runbook entry">
      <div data-testid="runbook-editor">
        <FormField label="Title">
          {(s) => (
            <input
              style={s}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          )}
        </FormField>
        <FormField label="Content (markdown)">
          {(s) => (
            <textarea
              style={{ ...s, minHeight: 120, fontFamily: 'inherit' }}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              required
            />
          )}
        </FormField>
        <FormField label="Tags (comma-separated)">
          {(s) => (
            <input
              style={s}
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="oncall, deploy, rollback"
            />
          )}
        </FormField>
        {error && (
          <div
            data-testid="runbook-error"
            style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}
          >
            {error}
          </div>
        )}
        <div
          style={{
            display: 'flex',
            gap: 'var(--sp-2)',
            justifyContent: 'flex-end',
            marginTop: 'var(--sp-4)',
          }}
        >
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            onClick={submit}
            disabled={submitting}
          >
            {submitting ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}