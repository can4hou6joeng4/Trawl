<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { api } from "../services";

const emit = defineEmits<{
  (e: "file", path: string): void;
  (e: "notify", message: string, type: "success" | "error" | "info"): void;
}>();

const isDragging = ref(false);
const supportedExtensions = [".docx", ".pdf", ".jpg", ".jpeg", ".png"] as const;

function isSupportedFile(path: string) {
  const lowerPath = path.toLowerCase();
  return supportedExtensions.some((extension) => lowerPath.endsWith(extension));
}

let cleanupWailsDrop: (() => void) | null = null;

onMounted(async () => {
  try {
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      isDragging.value = false;
      if (paths && paths.length > 0) {
        const filePath = paths[0];
        if (isSupportedFile(filePath)) {
          emit("file", filePath);
        } else {
          emit("notify", "不支持的文件格式，请拖入 .docx / .pdf / 图片 文件", "error");
        }
      }
    }, true);
    cleanupWailsDrop = OnFileDropOff;
  } catch (e) {
    console.error("Wails drop init failed:", e);
  }
});

onUnmounted(() => {
  if (cleanupWailsDrop) cleanupWailsDrop();
});

async function handleSelectFile() {
  try {
    const file = await api.service.selectFile();
    if (file) emit("file", file);
  } catch (e) {
    console.error("Select file failed:", e);
    emit("notify", "选择文件失败", "error");
  }
}
</script>

<template>
  <div class="landing">
    <div class="landing-head">
      <div class="landing-mark">LEGAL·<b>EXTRACTOR</b></div>
      <div class="landing-sub">法律文书 · 结构化取证</div>
    </div>
    <div
      class="dropzone"
      :class="{ over: isDragging }"
      style="--wails-drop-target: drop"
      @click="handleSelectFile"
      @dragover.prevent="isDragging = true"
      @dragleave="isDragging = false"
    >
      <span class="dz-corner tl"></span>
      <span class="dz-corner tr"></span>
      <span class="dz-corner bl"></span>
      <span class="dz-corner br"></span>
      <svg class="dz-icon" xmlns="http://www.w3.org/2000/svg" width="34" height="34" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
      <span class="dz-main">拖入文书，或点击选择</span>
      <span class="dz-hint">.PDF · .DOCX · .JPG/.PNG　单文件 ≤ 50MB</span>
    </div>
    <div class="landing-foot">解析在本地完成 · 扫描件与图片将使用云端 OCR（需配置 Token）</div>
  </div>
</template>

<style scoped>
.landing {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 22px;
}
.landing-head {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.landing-mark {
  font-family: var(--font-mono);
  font-size: 26px;
  letter-spacing: 4px;
  color: var(--text-primary);
}
.landing-mark b {
  color: var(--accent-primary);
  font-weight: 700;
}
.landing-sub {
  font-size: 12.5px;
  color: var(--text-muted);
  letter-spacing: 1px;
}
.dropzone {
  width: 560px;
  max-width: 82%;
  height: 250px;
  border: 1.5px dashed var(--surface-border-strong);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  transition: all 0.18s;
  background: var(--surface-panel-soft);
  position: relative;
  overflow: hidden;
  cursor: pointer;
}
.dropzone:hover,
.dropzone.over,
.dropzone.wails-drop-target-active {
  border-color: var(--accent-primary);
  background: rgba(24, 178, 93, 0.07);
  box-shadow: 0 0 40px rgba(24, 178, 93, 0.12) inset;
}
.dz-icon {
  color: var(--text-muted);
  transition: color 0.18s;
}
.dropzone:hover .dz-icon,
.dropzone.over .dz-icon {
  color: var(--accent-primary);
}
.dz-main {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
}
.dz-hint {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-muted);
  letter-spacing: 1px;
}
.dz-corner {
  position: absolute;
  width: 18px;
  height: 18px;
  border-color: var(--accent-primary);
  border-style: solid;
  opacity: 0.7;
}
.dz-corner.tl { top: 10px; left: 10px; border-width: 2px 0 0 2px; }
.dz-corner.tr { top: 10px; right: 10px; border-width: 2px 2px 0 0; }
.dz-corner.bl { bottom: 10px; left: 10px; border-width: 0 0 2px 2px; }
.dz-corner.br { bottom: 10px; right: 10px; border-width: 0 2px 2px 0; }
.landing-foot {
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--text-faint);
}
</style>
