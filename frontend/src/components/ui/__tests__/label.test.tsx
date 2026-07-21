import { createRef } from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Label } from '../label';

describe('Label', () => {
  it('renders a label with secondary denseness defaults', () => {
    render(<Label htmlFor="ssid">SSID</Label>);

    const label = screen.getByText('SSID');
    expect(label.tagName).toBe('LABEL');
    expect(label).toHaveAttribute('for', 'ssid');
    expect(label.className).toContain('text-xs');
    expect(label.className).toContain('font-medium');
    expect(label.className).toContain('text-gray-500');
    expect(label.className).toContain('dark:text-gray-400');
  });

  it('forwards ref and merges className', () => {
    const ref = createRef<HTMLLabelElement>();
    render(
      <Label ref={ref} className="text-sm font-medium">
        Password
      </Label>,
    );

    expect(ref.current).toBeInstanceOf(HTMLLabelElement);
    expect(ref.current?.className).toContain('text-sm');
    expect(ref.current?.className).toContain('font-medium');
  });
});
