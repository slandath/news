# RSS Feed Reader

A lightweight web server that fetches RSS feeds, converts XML to JSON, and displays them on a webpage.

## Features

- Parse RSS feeds from any URL
- Server-side rendered with Handlebars templates
- Browse multiple feeds from a home page
- Sorted feed listing

## Tech Stack

[![@antfu/eslint-config](https://img.shields.io/badge/%40antfu%2Feslint--config-7.2.0-4b3a8a?style=for-the-badge)](https://github.com/antfu/eslint-config)
[![Cheerio](https://img.shields.io/badge/Cheerio-1.2-CD853F?style=for-the-badge&logo=cheerio&logoColor=white)](https://github.com/cheeriojs/cheerio)
[![ESLint](https://img.shields.io/badge/ESLint-9.39.2-4b3a8a?style=for-the-badge&logo=eslint&logoColor=white)](https://github.com/eslint/eslint)
[![Hono](https://img.shields.io/badge/Hono-4.11.5-%23E36002?style=for-the-badge&logo=hono&logoColor=white)](https://github.com/honojs/hono)
[![Pico CSS](https://img.shields.io/badge/Pico-2.1.1-yellow?style=for-the-badge&logo=css&logoColor=white)](https://github.com/picocss/pico)
[![rss-parser](https://img.shields.io/badge/rss--parser-3.13.0-black?style=for-the-badge&logo=rss&logoColor=white)](https://github.com/rbren/rss-parser)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.9.3-blue?style=for-the-badge&logo=typescript&logoColor=white)](https://github.com/microsoft/TypeScript)

## Getting Started

```bash
pnpm install
pnpm dev
```

Visit `http://localhost:4000` to browse RSS feeds.

### Environment Variables

Create a `.env` file in the project root (copy from `.env.example`):

```bash
cp .env.example .env
```

Available environment variables:

- `PORT` - Server port (default: 4000)
- `NODE_ENV` - Environment mode: `development` or `production`

The app automatically uses the `PORT` environment variable for deployment platforms like Railway, Heroku, and Render.

## Usage

- **Home page:** `http://localhost:4000` - View all available feeds
- **View feed:** `http://localhost:4000/feed?url=<RSS_URL>` - View a specific feed
