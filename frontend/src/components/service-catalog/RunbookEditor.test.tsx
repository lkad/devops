// RunbookEditor — POST a new runbook entry.
//
// Covers the happy path (form filled -> apiPost called -> onSaved + onClose),
// client-side validation, and server-error propagation.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { RunbookEditor } from './RunbookEditor';

vi.mock('../../api/client', () => ({
  apiPost: vi.fn(),
}));

import { apiPost } from '../../api/client';

describe('RunbookEditor', () => {
  beforeEach(() => {
    vi.mocked(apiPost).mockReset();
    vi.mocked(apiPost).mockResolvedValue({ id: 'rb-1' } as any);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the editor when open', () => {
    render(
      <RunbookEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    expect(screen.getByText(/Add runbook entry/i)).toBeInTheDocument();
  });

  it('renders nothing when closed', () => {
    render(
      <RunbookEditor
        open={false}
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    expect(screen.queryByTestId('runbook-editor')).toBeNull();
  });

  it('submits form with title + content and calls onSaved + onClose', async () => {
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <RunbookEditor
        open
        onClose={onClose}
        serviceID="svc-1"
        onSaved={onSaved}
      />,
    );
    const editor = screen.getByTestId('runbook-editor');
    const titleInput = editor.querySelector(
      'input',
    ) as HTMLInputElement;
    const contentArea = editor.querySelector(
      'textarea',
    ) as HTMLTextAreaElement;
    fireEvent.change(titleInput, { target: { value: 'Restart procedure' } });
    fireEvent.change(contentArea, {
      target: { value: '1. ssh in\n2. systemctl restart' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(apiPost).toHaveBeenCalledWith(
        '/api/v1/services/svc-1/runbook',
        expect.objectContaining({
          title: 'Restart procedure',
          content: '1. ssh in\n2. systemctl restart',
        }),
      );
    });
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it('splits comma-separated tags into an array', async () => {
    render(
      <RunbookEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    const editor = screen.getByTestId('runbook-editor');
    const inputs = Array.from(
      editor.querySelectorAll('input'),
    ) as HTMLInputElement[];
    const titleInput = inputs[0];
    const tagsInput = inputs[1];
    const contentArea = editor.querySelector(
      'textarea',
    ) as HTMLTextAreaElement;
    fireEvent.change(titleInput, { target: { value: 't' } });
    fireEvent.change(contentArea, { target: { value: 'c' } });
    fireEvent.change(tagsInput, { target: { value: 'oncall, deploy, rollback' } });
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(apiPost).toHaveBeenCalledWith(
        '/api/v1/services/svc-1/runbook',
        expect.objectContaining({
          tags: ['oncall', 'deploy', 'rollback'],
        }),
      );
    });
  });

  it('shows a validation error when required fields are missing', async () => {
    render(
      <RunbookEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(screen.getByTestId('runbook-error')).toBeInTheDocument();
    });
    expect(apiPost).not.toHaveBeenCalled();
  });

  it('shows server error message when apiPost throws', async () => {
    vi.mocked(apiPost).mockRejectedValueOnce(new Error('server-fail'));
    render(
      <RunbookEditor
        open
        onClose={vi.fn()}
        serviceID="svc-1"
        onSaved={vi.fn()}
      />,
    );
    const editor = screen.getByTestId('runbook-editor');
    const titleInput = editor.querySelector(
      'input',
    ) as HTMLInputElement;
    const contentArea = editor.querySelector(
      'textarea',
    ) as HTMLTextAreaElement;
    fireEvent.change(titleInput, { target: { value: 't' } });
    fireEvent.change(contentArea, { target: { value: 'c' } });
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() => {
      expect(screen.getByTestId('runbook-error').textContent).toMatch(/server-fail/);
    });
  });
});