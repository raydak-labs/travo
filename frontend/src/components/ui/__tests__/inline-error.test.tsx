import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { InlineError } from '../inline-error';

describe('InlineError', () => {
  it('exposes an alert in the danger tone', () => {
    render(<InlineError>Failed to load status</InlineError>);

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Failed to load status');
    expect(alert.className).toContain('text-sm');
    // Light and dark chrome now come from one danger token set (ADR 0012),
    // so the alert cannot be half-converted and lose its dark variant.
    expect(alert.className).toContain('var(--status-danger-border)');
    expect(alert.className).toContain('var(--status-danger-surface)');
    expect(alert.className).toContain('var(--status-danger-text)');
  });

  it('merges className overrides', () => {
    render(<InlineError className="mt-2">Oops</InlineError>);

    expect(screen.getByRole('alert').className).toContain('mt-2');
  });
});
