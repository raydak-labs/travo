import { createRawSnippet } from 'svelte';
import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import Notice, { type NoticeTone } from './Notice.svelte';

const snippet = (html: string) => createRawSnippet(() => ({ render: () => html }));

function notice(props: { tone: NoticeTone; title?: string; actions?: string; role?: string }) {
  return render(Notice, {
    props: {
      ...props,
      children: snippet('<span>Router time zone is UTC</span>'),
      actions: props.actions ? snippet(props.actions) : undefined,
    },
  }).body.replace(/<!--[^>]*-->/g, '');
}

const cases: [NoticeTone, string, string][] = [
  ['info', 'lucide-info', 'status'],
  ['ok', 'lucide-circle-check', 'status'],
  ['warn', 'lucide-triangle-alert', 'alert'],
  ['danger', 'lucide-circle-alert', 'alert'],
];

describe('Notice', () => {
  it.each(cases)('%s shows its icon, no status dot, and role %s', (tone, icon, role) => {
    const body = notice({ tone });
    expect(body).toMatch(new RegExp(`<svg[^>]*${icon}[^>]*aria-hidden="true"`));
    expect(body).toContain(`text-${tone}`);
    expect(body).toContain(`role="${role}"`);
    expect(body).not.toMatch(/class="[^"]*\bstatus\b/);
    expect(body).toContain('Router time zone is UTC');
  });

  it('renders the title and the actions after the text', () => {
    const body = notice({
      tone: 'info',
      title: 'Time zone differs',
      actions: '<button>Update time zone</button>',
    });
    expect(body).toMatch(/data-slot="alert-title" class="[^"]*">Time zone differs</);
    expect(body).toMatch(
      /Router time zone is UTC[\s\S]*data-slot="alert-action" class="[^"]*">[\s\S]*<button>Update time zone<\/button>/,
    );
  });

  it('leaves out the action slot without actions and lets the caller set the role', () => {
    const body = notice({ tone: 'warn', role: 'note' });
    expect(body).not.toContain('data-slot="alert-action"');
    expect(body).toContain('role="note"');
  });
});
