import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PageSection } from '../page-section';

describe('PageSection', () => {
  it('is closed by default so content is not visible', () => {
    render(
      <PageSection title="DNS tools">
        <div>section body</div>
      </PageSection>,
    );

    expect(screen.getByRole('button', { name: /DNS tools/i })).toBeInTheDocument();
    expect(screen.queryByText('section body')).not.toBeInTheDocument();
  });

  it('opens content when the trigger is clicked', async () => {
    const user = userEvent.setup();
    render(
      <PageSection title="DNS tools">
        <div>section body</div>
      </PageSection>,
    );

    await user.click(screen.getByRole('button', { name: /DNS tools/i }));

    expect(screen.getByText('section body')).toBeVisible();
  });
});
