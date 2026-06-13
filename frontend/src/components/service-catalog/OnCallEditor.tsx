// OnCallEditor — modal form to create a new on-call rotation entry.
//
// POSTs to /api/v1/services/:serviceID/oncall. On success calls
// onSaved() and onClose() so the caller can refresh the
// detail view. The api client path starts with a leading slash
// (unlike useApi's relative paths) because this component is
// invoked directly — apiPost already prepends the api/v1
// prefix, so the slash lands us at /api/v1/services/:id/oncall.

import { useState } from 'react';
import { Modal } from '../common/Modal';
import { Button } from '../common/Button';
import { FormField } from '../common/FormField';
import { apiPost } from '../../api/client';

export interface OnCallEditorProps {
  open: boolean;
  onClose: () => void;
  serviceID: string;
  onSaved: () => void;
}

export function OnCallEditor({ open, onClose, serviceID, onSaved }: OnCallEditorProps) {
  const [userID, setUserID] = useState('');
  const [shiftStart, setShiftStart] = useState('');
  const [shiftEnd, setShiftEnd] = useState('');
  const [timezone, setTimezone] = useState('UTC');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const submit = async () => {
    if (!userID || !shiftStart || !shiftEnd) {
      setError('user_id, shift_start, shift_end are required');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await apiPost(`/api/v1/services/${serviceID}/oncall`, {
        user_id: userID,
        shift_start: shiftStart,
        shift_end: shiftEnd,
        timezone,
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
    <Modal open={open} onClose={onClose} title="Add on-call rotation">
      <div data-testid="on-call-editor">
        <FormField label="User ID">
          {(s) => (
            <input
              style={s}
              value={userID}
              onChange={(e) => setUserID(e.target.value)}
              required
            />
          )}
        </FormField>
        <FormField label="Shift start (RFC3339)">
          {(s) => (
            <input
              style={s}
              value={shiftStart}
              onChange={(e) => setShiftStart(e.target.value)}
              placeholder="2026-06-13T08:00:00Z"
              required
            />
          )}
        </FormField>
        <FormField label="Shift end (RFC3339)">
          {(s) => (
            <input
              style={s}
              value={shiftEnd}
              onChange={(e) => setShiftEnd(e.target.value)}
              placeholder="2026-06-13T17:00:00Z"
              required
            />
          )}
        </FormField>
        <FormField label="Timezone">
          {(s) => (
            <input
              style={s}
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
            />
          )}
        </FormField>
        {error && (
          <div
            data-testid="on-call-error"
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