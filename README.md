# RSS Feed Reader

A lightweight web server that fetches RSS feeds, converts XML to JSON, and displays them on a webpage.

## Features

- Parse RSS feeds from any URL
- Server-side rendered with Nunjucks templates
- Browse multiple feeds from a home page
- Sorted feed listing
- Lightweight and fast

## Tech Stack

[![@antfu/eslint-config](https://img.shields.io/badge/%40antfu%2Feslint--config-7.2.0-4b3a8a?style=for-the-badge)](https://github.com/antfu/eslint-config)
[![ESLint](https://img.shields.io/badge/ESLint-9.39.2-4b3a8a?style=for-the-badge&logo=eslint&logoColor=white)](https://github.com/eslint/eslint)
[![Hono](https://img.shields.io/badge/Hono-4.11.5-blue?style=for-the-badge&logo=hono&logoColor=white)](https://github.com/honojs/hono)
[![Nunjucks](https://img.shields.io/badge/Nunjucks-3.2.4-green?style=for-the-badge&logo=mozilla&logoColor=white)](https://github.com/mozilla/nunjucks)
[![rss-parser](https://img.shields.io/badge/rss--parser-3.13.0-orange?style=for-the-badge&logo=rss&logoColor=white)](https://github.com/rbren/rss-parser)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.9.3-blue?style=for-the-badge&logo=typescript&logoColor=white)](https://github.com/microsoft/TypeScript)

## Getting Started

```bash
pnpm install
pnpm dev
```

Visit `http://localhost:4000` to browse RSS feeds.

## Usage

- **Home page:** `http://localhost:4000` - View all available feeds
- **View feed:** `http://localhost:4000/feed?url=<RSS_URL>` - View a specific feed
