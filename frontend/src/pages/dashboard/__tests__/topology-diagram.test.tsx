import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Cable, Wifi } from 'lucide-react';
import { TopologyDiagram } from '../topology-diagram';
import type { SourceDef } from '@/hooks/use-topology-data';

const ethernet: SourceDef = {
  key: 'wan',
  label: 'Ethernet',
  icon: Cable,
  connected: true,
  interfaceName: 'wan',
  rate: { rx: 625_000, tx: 125_000 },
};

const repeater: SourceDef = {
  key: 'wwan',
  label: 'Repeater (WiFi)',
  icon: Wifi,
  connected: false,
  interfaceName: 'wwan0',
  rate: { rx: 1_250_000, tx: 250_000 },
};

function renderDiagram(sources: SourceDef[]) {
  return render(
    <TopologyDiagram
      sources={sources}
      clients={[]}
      hostname="travo"
      model="GL-MT3000"
      features={[]}
      loading={false}
    />,
  );
}

describe('TopologyDiagram traffic hints', () => {
  it('annotates the connected uplink with its live rate', () => {
    renderDiagram([ethernet, repeater]);

    // The diagram renders a stacked and a wide layout; both carry the hint.
    expect(screen.getAllByText('Download 5.0 Mbit/s, upload 1.0 Mbit/s')).toHaveLength(2);
    expect(screen.queryByText('Download 10.0 Mbit/s, upload 2.0 Mbit/s')).not.toBeInTheDocument();
  });

  it('does not annotate a source before a rate exists', () => {
    renderDiagram([{ ...ethernet, rate: null }]);

    expect(screen.queryByText(/Download .* upload/)).not.toBeInTheDocument();
  });
});
