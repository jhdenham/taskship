// Sidebar order and labels for docs/*.md. Every file in docs/ must be listed
// here exactly once; the build fails otherwise.
export type NavItem = { id: string; label: string; planned?: boolean };

export const nav: { group: string; items: NavItem[] }[] = [
  {
    group: 'Start here',
    items: [
      { id: 'overview', label: 'Overview' },
      { id: 'quickstart', label: 'Quickstart' },
      { id: 'concepts', label: 'Concepts' },
    ],
  },
  {
    group: 'Guides',
    items: [
      { id: 'tui', label: 'Terminal UI' },
      { id: 'self-hosting', label: 'Self-hosting', planned: true },
    ],
  },
  {
    group: 'Extend',
    items: [{ id: 'integrations', label: 'Integrations', planned: true }],
  },
  {
    group: 'Project',
    items: [{ id: 'roadmap', label: 'Roadmap' }],
  },
];

export const navItems = nav.flatMap((g) => g.items);

export function docHref(id: string): string {
  return id === 'overview' ? '/docs/' : `/docs/${id}/`;
}
