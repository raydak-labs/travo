import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PageSection } from '../page-section';
import { Card, CardContent, CardHeader, CardTitle } from '../card';

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

  it('uses card chrome and header padding on the collapsed trigger', () => {
    const { container } = render(
      <PageSection title="DNS tools">
        <div>section body</div>
      </PageSection>,
    );

    const shell = container.firstElementChild as HTMLElement;
    expect(shell.className).toMatch(/rounded-lg/);
    expect(shell.className).toMatch(/border/);

    const trigger = screen.getByRole('button', { name: /DNS tools/i });
    expect(trigger.className).toMatch(/px-6/);
    expect(trigger.className).toMatch(/py-4/);
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

  it('hides nested CardHeader when expanded so titles are not duplicated', async () => {
    const user = userEvent.setup();
    render(
      <PageSection title="Firewall">
        <Card>
          <CardHeader>
            <CardTitle>Firewall</CardTitle>
          </CardHeader>
          <CardContent>rules body</CardContent>
        </Card>
      </PageSection>,
    );

    await user.click(screen.getByRole('button', { name: /Firewall/i }));

    expect(screen.getByText('rules body')).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Firewall' })).not.toBeInTheDocument();
  });
});
