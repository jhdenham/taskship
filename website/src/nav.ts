// Sidebar order and labels for docs/*.md. Every file in docs/ must be listed
// here exactly once; the build fails otherwise.
export type NavItem = { id: string; label: string };

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
    group: 'Tasks',
    items: [
      { id: 'tasks', label: 'Tasks' },
      { id: 'projects', label: 'Projects' },
      { id: 'tui', label: 'Terminal UI' },
    ],
  },
  {
    group: 'Agents',
    items: [
      { id: 'launching', label: 'Launching agents' },
      { id: 'history', label: 'Run history' },
      { id: 'skill', label: 'Skill' },
      { id: 'conventions', label: 'Conventions' },
    ],
  },
  {
    group: 'Planned',
    items: [
      { id: 'shared-server', label: 'Shared server' },
      { id: 'integrations', label: 'Integrations' },
      { id: 'roadmap', label: 'Roadmap' },
    ],
  },
];

export const navItems = nav.flatMap((g) => g.items);

export function docHref(id: string): string {
  return id === 'overview' ? '/docs/' : `/docs/${id}/`;
}
