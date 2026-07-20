import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { InlineError } from '../inline-error';

describe('InlineError', () => {
  it('exposes an alert with red light/dark chrome', () => {
    render(<InlineError>Failed to load status</InlineError>);

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Failed to load status');
    expect(alert.className).toContain('text-sm');
    expect(alert.className).toContain('border-red');
    expect(alert.className).toContain('bg-red');
    expect(alert.className).toContain('dark:border-red');
    expect(alert.className).toContain('dark:bg-red');
    expect(alert.className).toContain('text-red');
    expect(alert.className).toContain('dark:text-red');
  });

  it('merges className overrides', () => {
    render(<InlineError className="mt-2">Oops</InlineError>);

    expect(screen.getByRole('alert').className).toContain('mt-2');
  });
});
