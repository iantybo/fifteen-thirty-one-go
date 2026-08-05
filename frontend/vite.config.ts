/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // Vitest configuration for the Optimistic Sync Engine unit tests. The engine,
  // reducer, and queue are pure/DI-driven, so a plain node environment is
  // sufficient (no jsdom required).
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
})
