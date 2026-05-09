// SPDX-FileCopyrightText: 2023-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
//
// SPDX-License-Identifier: EUPL-1.2

import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { docsSchema } from '@astrojs/starlight/schema';

const docs = defineCollection({
  loader: glob({ pattern: "**/*.md", base: "../patterns" }),
  schema: docsSchema()
});

export const collections = { docs };
