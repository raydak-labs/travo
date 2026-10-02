import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Card, CardHeader, CardTitle } from '../card';

describe('CardTitle', () => {
  it('uses compact title defaults', () => {
    render(<CardTitle>Settings</CardTitle>);

    const title = screen.getByRole('heading', { level: 3, name: 'Settings' });
    expect(title.className).toContain('text-sm');
    expect(title.className).toContain('font-medium');
    expect(title.className).toContain('leading-none');
    expect(title.className).toContain('tracking-tight');
    expect(title.className).not.toContain('text-lg');
    expect(title.className).not.toContain('font-semibold');
  });

  it('renders its children and forwards the heading level contract', () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Settings</CardTitle>
        </CardHeader>
      </Card>,
    );

    const title = screen.getByRole('heading', { level: 3, name: 'Settings' });
    expect(title).toBeInTheDocument();
    expect(title.closest('[data-slot="card-header"]')).not.toBeNull();
  });

  it('merges className overrides', () => {
    render(<CardTitle className="flex items-center gap-2">With icon</CardTitle>);

    const title = screen.getByRole('heading', { level: 3, name: 'With icon' });
    expect(title.className).toContain('flex');
    expect(title.className).toContain('items-center');
    expect(title.className).toContain('gap-2');
    expect(title.className).toContain('text-sm');
  });
});
