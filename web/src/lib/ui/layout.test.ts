import Inbox from '@lucide/svelte/icons/inbox';
import { createRawSnippet } from 'svelte';
import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import EmptyState from './EmptyState.svelte';
import GroupHeading from './GroupHeading.svelte';
import PageHeader from './PageHeader.svelte';
import Section from './Section.svelte';
import SettingsRow from './SettingsRow.svelte';

const snippet = (html: string) => createRawSnippet(() => ({ render: () => html }));
const tags = (body: string, tag: string) =>
  body.match(new RegExp(`<${tag}[\\s>]`, 'g'))?.length ?? 0;

describe('PageHeader', () => {
  it('renders the one h1 with its description and actions', () => {
    const { body } = render(PageHeader, {
      props: {
        title: 'WiFi',
        description: 'About WiFi',
        actions: snippet('<button>Refresh</button>'),
      },
    });
    expect(tags(body, 'h1')).toBe(1);
    expect(body).toMatch(/<h1[^>]*>WiFi<\/h1>/);
    expect(body).toContain('About WiFi');
    expect(body).toContain('<button>Refresh</button>');
    expect(tags(body, 'h2')).toBe(0);
  });

  it('leaves out description and actions when not given', () => {
    const { body } = render(PageHeader, { props: { title: 'WiFi' } });
    expect(body).not.toContain('<p');
    expect(body).not.toContain('<button');
  });
});

describe('Section', () => {
  it('renders one card surface with an h2 that labels the region', () => {
    const { body } = render(Section, {
      props: {
        title: 'Saved uplinks',
        description: 'Rejoined first',
        children: snippet('<p>ROWS</p>'),
      },
    });
    expect(tags(body, 'h1')).toBe(0);
    expect(tags(body, 'h2')).toBe(1);
    const id = body.match(/<h2 id="([^"]+)"/)?.[1];
    expect(id).toBeTruthy();
    expect(body).toContain(`aria-labelledby="${id}"`);
    expect(body).toContain('role="region"');
    expect(body).toContain('data-slot="card"');
    expect(body).toContain('Rejoined first');
    expect(body).toContain('<p>ROWS</p>');
  });

  it('puts actions in the card action slot', () => {
    const { body } = render(Section, {
      props: {
        title: 'Nearby',
        actions: snippet('<button>Scan</button>'),
        children: snippet('<p></p>'),
      },
    });
    expect(body).toMatch(/data-slot="card-action"[^>]*><button>Scan<\/button>/);
  });
});

describe('SettingsRow', () => {
  it('associates the label with its control by id', () => {
    const { body } = render(SettingsRow, {
      props: { label: 'Custom MAC', for: 'mac', control: snippet('<input id="mac">') },
    });
    expect(body.replace(/<!--[^>]*-->/g, '')).toMatch(
      /<label[^>]*for="mac"[^>]*>Custom MAC<\/label>/,
    );
    expect(body).toContain('<input id="mac">');
  });

  it('shows the label as text without a control id, and the description', () => {
    const { body } = render(SettingsRow, {
      props: {
        label: 'Address',
        description: 'Policy: default',
        control: snippet('<button>Random</button>'),
      },
    });
    expect(body).not.toContain('<label');
    expect(body).toContain('Address');
    expect(body).toContain('Policy: default');
    expect(body).toContain('data-slot="settings-row"');
  });
});

describe('EmptyState', () => {
  it('renders the icon hidden, the title, text and action', () => {
    const { body } = render(EmptyState, {
      props: {
        icon: Inbox,
        title: 'No saved uplinks',
        text: 'Save one',
        action: snippet('<button>Scan</button>'),
      },
    });
    expect(body).toContain('aria-hidden="true"');
    expect(body).toContain('No saved uplinks');
    expect(body).toContain('Save one');
    expect(body).toContain('<button>Scan</button>');
    expect(body).not.toContain('role="alert"');
  });
});

describe('GroupHeading', () => {
  it('is an h3', () => {
    const { body } = render(GroupHeading, { props: { label: 'Guest network' } });
    expect(body).toMatch(/<h3[^>]*>Guest network<\/h3>/);
  });
});
