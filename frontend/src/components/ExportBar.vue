<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import type { ExportFormat } from "../utils/fields";

defineProps<{
  outputPath: string;
  format: ExportFormat;
  exporting: boolean;
  done: { path: string; count: number; edited: boolean } | null;
}>();

const emit = defineEmits<{
  (e: "update:format", value: ExportFormat): void;
  (e: "choose-path"): void;
  (e: "export"): void;
  (e: "open-file"): void;
  (e: "dismiss-done"): void;
}>();

const FORMATS: ExportFormat[] = ["xlsx", "csv", "json"];
const fmtOpen = ref(false);
const rootEl = ref<HTMLElement | null>(null);

function pick(f: ExportFormat) {
  emit("update:format", f);
  fmtOpen.value = false;
}

function onOutside(e: PointerEvent) {
  if (fmtOpen.value && rootEl.value && !rootEl.value.contains(e.target as Node)) {
    fmtOpen.value = false;
  }
}

onMounted(() => document.addEventListener("pointerdown", onOutside, true));
onUnmounted(() => document.removeEventListener("pointerdown", onOutside, true));
</script>

<template>
  <div ref="rootEl" class="exportbar">
    <div v-if="done" class="export-done" role="status" aria-live="polite">
      <span class="ok">
        <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
      </span>
      <div class="done-text">
        <div class="t">已导出 {{ done.count }} 条记录{{ done.edited ? "（含人工修正）" : "" }}</div>
        <div class="d">{{ done.path }}</div>
      </div>
      <button class="open" type="button" @click="emit('open-file')">打开文件</button>
      <button class="x" type="button" aria-label="关闭" @click="emit('dismiss-done')">✕</button>
    </div>

    <span class="exp-path" :title="outputPath">
      <span class="pp">{{ outputPath || "尚未选择保存位置" }}</span>
      <button class="chg" type="button" @click="emit('choose-path')">{{ outputPath ? "更改" : "选择" }}</button>
    </span>

    <button class="btn-export" type="button" :disabled="exporting" @click="emit('export')">
      {{ exporting ? "导出中…" : "导出" }}
      <span class="fmt" @click.stop="fmtOpen = !fmtOpen">{{ format.toUpperCase() }} ▾</span>
    </button>

    <div v-if="fmtOpen" class="fmt-pop">
      <button
        v-for="f in FORMATS"
        :key="f"
        type="button"
        :class="{ on: f === format }"
        @click="pick(f)"
      >{{ f.toUpperCase() }}<span>{{ f === format ? "✓" : "" }}</span></button>
    </div>
  </div>
</template>

<style scoped>
.exportbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-top: 1px solid var(--surface-border);
  flex-shrink: 0;
  position: relative;
}
.exp-path {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
  min-width: 0;
}
.exp-path .pp {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.exp-path .chg {
  color: var(--accent-soft);
  background: none;
  border: none;
  font-family: inherit;
  font-size: inherit;
  cursor: pointer;
  flex-shrink: 0;
}
.exp-path .chg:hover {
  color: var(--accent-primary);
}
.btn-export {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 9px;
  background: var(--accent-primary);
  color: #04170c;
  font-weight: 800;
  font-size: 12.5px;
  padding: 10px 16px;
  border: none;
  border-radius: 8px;
  flex-shrink: 0;
  cursor: pointer;
  box-shadow: 0 4px 24px rgba(52, 216, 121, 0.35);
  transition: all 0.15s;
  font-family: var(--font-body);
}
.btn-export:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 6px 28px rgba(52, 216, 121, 0.45);
}
.btn-export:disabled {
  opacity: 0.6;
  cursor: default;
}
.btn-export .fmt {
  font-family: var(--font-mono);
  background: rgba(4, 23, 12, 0.18);
  padding: 2px 7px;
  border-radius: 4px;
  font-size: 10.5px;
}
.fmt-pop {
  position: absolute;
  bottom: calc(100% + 6px);
  right: 16px;
  background: #0d1a12;
  border: 1px solid var(--surface-border-strong);
  border-radius: 9px;
  padding: 5px;
  z-index: 30;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.6);
}
.fmt-pop button {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  width: 100%;
  text-align: left;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-secondary);
  padding: 7px 12px;
  border-radius: 5px;
  background: none;
  border: none;
  cursor: pointer;
}
.fmt-pop button:hover {
  background: rgba(24, 178, 93, 0.1);
}
.fmt-pop button.on {
  color: var(--accent-primary);
}
.export-done {
  position: absolute;
  bottom: calc(100% + 10px);
  left: 16px;
  right: 16px;
  background: rgba(13, 26, 18, 0.97);
  border: 1px solid rgba(52, 216, 121, 0.5);
  border-radius: 10px;
  padding: 11px 14px;
  display: flex;
  align-items: center;
  gap: 11px;
  box-shadow: 0 16px 44px rgba(0, 0, 0, 0.55);
  animation: rise 0.25s ease-out;
  z-index: 20;
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
}
.export-done .ok {
  width: 26px;
  height: 26px;
  border-radius: 7px;
  background: rgba(52, 216, 121, 0.15);
  color: var(--accent-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.export-done .done-text {
  min-width: 0;
}
.export-done .t {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-primary);
}
.export-done .d {
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.export-done .open {
  margin-left: auto;
  font-size: 11px;
  color: var(--accent-primary);
  border: 1px solid rgba(52, 216, 121, 0.45);
  background: none;
  padding: 6px 12px;
  border-radius: 7px;
  flex-shrink: 0;
  cursor: pointer;
  font-family: var(--font-body);
}
.export-done .open:hover {
  background: rgba(52, 216, 121, 0.1);
}
.export-done .x {
  color: var(--text-muted);
  font-size: 13px;
  flex-shrink: 0;
  padding: 4px;
  background: none;
  border: none;
  cursor: pointer;
}
</style>
