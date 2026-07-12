import {defineConfig, type Plugin} from 'vite'
import vue from '@vitejs/plugin-vue'
import fs from 'node:fs'
import path from 'node:path'

// pdf.js 运行时按需加载的 CJK 字符映射与标准字体：
// 构建时拷入产物 dist/pdfjs/（不进版本库），dev 下由中间件直读 node_modules。
function pdfAssets(): Plugin {
  const dirs = ['cmaps', 'standard_fonts']
  const srcRoot = path.resolve(__dirname, 'node_modules/pdfjs-dist')
  return {
    name: 'pdf-assets',
    closeBundle() {
      for (const dir of dirs) {
        fs.cpSync(path.join(srcRoot, dir), path.resolve(__dirname, 'dist/pdfjs', dir), {recursive: true})
      }
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = (req.url || '').split('?')[0]
        if (!url.startsWith('/pdfjs/')) return next()
        const rel = path.normalize(url.slice('/pdfjs/'.length))
        if (rel.startsWith('..') || !dirs.includes(rel.split(path.sep)[0])) return next()
        const file = path.join(srcRoot, rel)
        if (!fs.existsSync(file) || !fs.statSync(file).isFile()) return next()
        res.setHeader('Content-Type', 'application/octet-stream')
        fs.createReadStream(file).pipe(res)
      })
    },
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), pdfAssets()],
})
