import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// 最小化 vitest 配置：复用 vite 的 vue 插件以便后续测组件，
// 现阶段仅需 jsdom 环境跑 utils/services 单测。
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: false,
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['src/**/*.ts'],
      exclude: ['src/**/*.{test,spec}.ts', 'src/main.ts', 'src/env.d.ts'],
    },
  },
})
