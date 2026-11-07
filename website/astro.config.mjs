import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://taskship.dev',
  trailingSlash: 'always',
  markdown: {
    // Plain code blocks for now, styled like the terminal in global.css.
    syntaxHighlight: false,
  },
});
