// OnCallEditor — POST a new on-call rotation entry.
//
// Covers the happy path (form filled -> apiPost called -> onSaved + onClose),
// client-side validation (missing fields -> error shown), and server-error
// propagation. The api client is mocked so the test runs offline.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { OnCallEditor } from './OnCallEditor';

vi.mock('../../api/client', () => ({
  apiPost: vi.fn(),
}));

import { apiPost } from '../../api/client';

describe('OnCallEditor', () => {
  beforeEach(() => {
    vi.mocked(apiPost).mockReset();
    vi.mocked(apiPost).mockResolvedValue({ id: 'shift-1' } as any);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the editor when open', () => {
    render(
      <OnCallEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    expect(screen.getByText(/Add on-call rotation/i)).toBeInTheDocument();
  });

  it('renders nothing when closed', () => {
    render(
      <OnCallEditor
        open={false}
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    expect(screen.queryByTestId('on-call-editor')).toBeNull();
  });

  it('submits form with all fields filled and calls onSaved + onClose', async () => {
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <OnCallEditor
        open
        onClose={onClose}
        serviceID="svc-1"
        onSaved={onSaved}
      />,
    );
    // The FormField pattern does not associate the
    // <label> with the <input> via htmlFor, so we
    // locate fields by their input's aria-label or
    // by container scoped to the editor testid.
    const editor = screen.getByTestId('on-call-editor');
    const userInput = editor.querySelector(
      'input[placeholder=""], input:not([placeholder])',
    ) as HTMLInputElement;
    // The first bare input is the user ID (no placeholder).
    const inputs = Array.from(
      editor.querySelectorAll('input'),
    ) as HTMLInputElement[];
    fireEvent.change(inputs[0], { target: { value: 'alice' } });
    fireEvent.change(inputs[1], { target: { value: '2026-06-13T08:00:00Z' } });
    fireEvent.change(inputs[2], { target: { value: '2026-06-13T17:00:00Z' } });
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(apiPost).toHaveBeenCalledWith(
        '/api/v1/services/svc-1/oncall',
        expect.objectContaining({
          user_id: 'alice',
          shift_start: '2026-06-13T08:00:00Z',
          shift_end: '2026-06-13T17:00:00Z',
        }),
      );
    });
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('shows a validation error when required fields are missing', async () => {
    render(
      <OnCallEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(screen.getByTestId('on-call-error')).toBeInTheDocument();
    });
    expect(apiPost).not.toHaveBeenCalled();
  });

  it('shows server error message when apiPost throws', async () => {
    vi.mocked(apiPost).mockRejectedValueOnce(new Error('boom'));
    render(
      <OnCallEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    const editor = screen.getByTestId('on-call-editor');
    const inputs = Array.from(
      editor.querySelectorAll('input'),
    ) as HTMLInputElement[];
    fireEvent.change(inputs[0], { target: { value: 'alice' } });
    fireEvent.change(inputs[1], { target: { value: '2026-06-13T08:00:00Z' } });
    fireEvent.change(inputs[2], { target: { value: '2026-06-13T17:00:00Z' } });
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(screen.getByTestId('on-call-error').textContent).toMatch(/boom/);
    });
  });
});