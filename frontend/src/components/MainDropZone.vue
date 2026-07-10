<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { api } from "../services";
import { OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";

const props = defineProps<{
  selectedFile: string | null;
  fileName: string;
}>();

const displayPath = computed(() => {
  if (!props.selectedFile) return "";
  return props.selectedFile;
});

const emit = defineEmits<{
  (e: "update:selectedFile", value: string): void;
  (
    e: "notification",
    message: string,
    type: "success" | "error" | "info"
  ): void;
}>();

const isDragging = ref(false);
const supportedExtensions = [".docx", ".pdf", ".jpg", ".jpeg", ".png"] as const;

function setFile(file: string) {
  emit("update:selectedFile", file);
}

function isSupportedFile(path: string) {
  const lowerPath = path.toLowerCase();
  return supportedExtensions.some((extension) => lowerPath.endsWith(extension));
}

// 桌面版：Wails 原生拖拽处理
let cleanupWailsDrop: (() => void) | null = null;

onMounted(async () => {
  try {
    OnFileDrop((x: number, y: number, paths: string[]) => {
      isDragging.value = false;
      if (paths && paths.length > 0) {
        const filePath = paths[0];
        if (isSupportedFile(filePath)) {
          setFile(filePath);
          emit("notification", "文件已加载", "success");
        } else {
          emit("notification", "不支持的文件格式", "error");
        }
      }
    }, true);

    cleanupWailsDrop = OnFileDropOff;
  } catch (e) {
    console.warn("Wails runtime not available:", e);
  }
});

onUnmounted(() => {
  if (cleanupWailsDrop) {
    cleanupWailsDrop();
  }
});

async function handleSelectFile() {
  try {
    const file = await api.service.selectFile();
    if (file) {
      setFile(file);
    }
  } catch (e) {
    console.error("File selection failed:", e);
    // 用户取消选择不报错
    if ((e as Error).message !== "未选择文件") {
       emit("notification", "选择文件失败", "error");
    }
  }
}
</script>

<template>
  <div
    class="drop-zone"
    :class="{ 'is-dragging': isDragging, 'has-file': !!selectedFile }"
    style="--wails-drop-target: drop"
    @click="handleSelectFile"
    @dragover.prevent="isDragging = true"
    @dragleave.prevent="isDragging = false"
  >
    <div class="drop-content">
      <div class="icon-wrapper">
        <div v-if="!selectedFile" class="icon-svg">
            <!-- Folder Icon -->
            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>
        </div>
        <div v-else class="icon-svg">
            <!-- File Icon -->
            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/></svg>
        </div>
      </div>
      <div class="text-content">
        <h3 v-if="!selectedFile">选择待处理文书</h3>
        <div v-else class="selected-file-info">
          <h3 class="file-name-display">{{ fileName }}</h3>
          <p class="file-path-text" :title="String(selectedFile)">{{ displayPath }}</p>
        </div>
        <p v-if="!selectedFile" class="hint">点击选择或拖入 .docx / .pdf / .jpg / .jpeg / .png 文件</p>
      </div>
      <button v-if="selectedFile" class="change-file-btn">
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/></svg>
        <span>更换文件</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Drop Zone */
.drop-zone {
  border: 1px dashed rgba(148, 163, 184, 0.24);
  border-radius: 8px;
  padding: 20px;
  cursor: pointer;
  transition: all 0.2s ease;
  background:
    linear-gradient(135deg, rgba(34, 197, 94, 0.06), rgba(56, 189, 248, 0.04)),
    rgba(2, 6, 23, 0.34);
  position: relative;
  overflow: hidden;
}

.drop-zone::before {
  background: linear-gradient(90deg, rgba(34, 197, 94, 0.48), rgba(56, 189, 248, 0.24));
  content: "";
  height: 2px;
  left: 0;
  opacity: 0.65;
  position: absolute;
  right: 0;
  top: 0;
}

.drop-zone:hover,
.drop-zone.is-dragging,
.drop-zone.wails-drop-target-active {
  border-color: var(--accent-primary);
  background: rgba(34, 197, 94, 0.06);
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.08);
}

.drop-zone.has-file {
  border-style: solid;
  background: rgba(15, 23, 42, 0.62);
  border-color: rgba(34, 197, 94, 0.26);
}

.drop-content {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.icon-wrapper {
  width: 52px;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(148, 163, 184, 0.08);
  border: 1px solid rgba(148, 163, 184, 0.12);
  border-radius: 8px;
  color: var(--accent-primary);
  flex-shrink: 0;
  transition: transform 0.2s ease;
}

.drop-zone:hover .icon-wrapper {
  transform: translateY(-1px);
  background: rgba(34, 197, 94, 0.12);
}

.text-content {
  flex: 1;
  min-width: 0;
}

.text-content h3 {
  font-size: 1.04rem;
  font-weight: 600;
  color: var(--text-primary);
  font-family: var(--font-body);
  letter-spacing: 0;
}

.selected-file-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
  min-width: 0;
}

.file-name-display {
  color: var(--text-primary) !important;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-path-text {
  font-size: 0.8rem;
  color: var(--text-muted);
  font-family: var(--font-body);
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  opacity: 0.7;
}

.hint {
  color: var(--text-muted);
  font-size: 0.82rem;
  margin-top: 2px;
}

.change-file-btn {
  margin-top: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 0.8rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.change-file-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--accent-primary);
  border-color: var(--accent-primary);
  transform: translateY(-1px);
}

.change-file-btn svg {
  transition: transform 0.5s ease;
}

@media (max-width: 640px) {
  .drop-zone {
    padding: 14px;
  }

  .drop-content {
    align-items: flex-start;
    gap: 12px;
  }

  .icon-wrapper {
    width: 42px;
    height: 42px;
  }

  .change-file-btn {
    padding: 8px;
  }

  .change-file-btn span {
    display: none;
  }
}
</style>
