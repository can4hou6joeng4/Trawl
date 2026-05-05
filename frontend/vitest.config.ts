import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// vitest 配置：jsdom + v8 覆盖率 + 按文件 glob 的覆盖率门槛。
//
// 阈值仅设给已写测试的文件——本轮不测的 App.vue / ConfigPanel.vue
// 不在阈值表，避免「假装在保护一切」。后续轮次补测哪个文件，
// 再把它加进 thresholds。
//
// 阈值数字策略：取「实测值 -5~10pp」作为防回归门槛，
// 保证现在 PASS、以后回退会红。
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: false,
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'lcov'],
      include: ['src/**/*.ts', 'src/**/*.vue'],
      exclude: [
        'src/**/*.{test,spec}.ts',
        'src/main.ts',
        'src/vite-env.d.ts',
        'src/services/index.ts',
        'src/test-utils/**',
      ],
      thresholds: {
        'src/utils/**/*.ts': {
          lines: 80, branches: 80, functions: 90, statements: 80,
        },
        'src/services/api.ts': {
          lines: 25, branches: 60, functions: 20, statements: 25,
        },
        'src/components/ResultCard.vue': {
          lines: 90, branches: 70, functions: 90, statements: 90,
        },
        'src/components/PreviewTable.vue': {
          lines: 90, branches: 85, functions: 50, statements: 90,
        },
        'src/components/MainDropZone.vue': {
          lines: 65, branches: 55, functions: 90, statements: 65,
        },
      },
    },
  },
})
