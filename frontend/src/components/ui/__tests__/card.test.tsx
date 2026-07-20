import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CardTitle } from '../card';

describe('CardTitle', () => {
  it('uses compact title defaults', () => {
    render(<CardTitle>Settings</CardTitle>);

    const title = screen.getByRole('heading', { level: 3, name: 'Settings' });
    expect(title.className).toContain('text-sm');
    expect(title.className).toContain('font-medium');
    expect(title.className).toContain('leading-none');
    expect(title.className).toContain('tracking-tight');
    expect(title.className).toContain('text-gray-900');
    expect(title.className).toContain('dark:text-white');
    expect(title.className).not.toContain('text-lg');
    expect(title.className).not.toContain('font-semibold');
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
