import { defineConfig } from 'astro/config';
import { satteri } from '@astrojs/markdown-satteri';

// The docs link to each other as files (`tasks.md#labels`) so the links work
// on GitHub too. On the site, point them at the pages instead.
const docLinks = {
  name: 'doc-links',
  link(node, ctx) {
    const m = /^([\w-]+)\.md(#.*)?$/.exec(node.url);
    if (m) ctx.setProperty(node, 'url', (m[1] === 'overview' ? '/docs/' : `/docs/${m[1]}/`) + (m[2] ?? ''));
  },
};

export default defineConfig({
  site: 'https://taskship.dev',
  trailingSlash: 'always',
  markdown: {
    // Token colours come from the palette in global.css.
    shikiConfig: { theme: 'css-variables' },
    processor: satteri({ mdastPlugins: [docLinks] }),
  },
});
