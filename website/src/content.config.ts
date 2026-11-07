import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';

// The docs are the repository's docs/ folder, read in place. docs/ stays the
// single source of truth and is never copied into the website.
export const collections = {
  docs: defineCollection({
    loader: glob({ pattern: '*.md', base: '../docs' }),
  }),
};
