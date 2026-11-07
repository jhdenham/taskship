import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://taskship.dev',
  trailingSlash: 'always',
  markdown: {
    // Token colours come from the palette in global.css.
    shikiConfig: { theme: 'css-variables' },
  },
});
