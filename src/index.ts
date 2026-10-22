// Import app and libraries
import process from 'node:process'
import { serve } from '@hono/node-server'
import { app } from './app.js'

// Start HTTP server on port
const PORT = Number(process.env.PORT) || 3000
serve({ fetch: app.fetch, port: PORT })
