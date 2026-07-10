<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { api, friendlyErrorMessage, isCancelledError, type Record, type ExtractResult, type TrialStatus } from "./services";
import { useNotification } from "./composables/useNotification";
import { useExtractionProgress } from "./composables/useExtractionProgress";
import MainDropZone from "./components/MainDropZone.vue";
import ConfigPanel from "./components/ConfigPanel.vue";
import ResultCard from "./components/ResultCard.vue";
import PreviewTable from "./components/PreviewTable.vue";
import TrialBanner from "./components/TrialBanner.vue";
import ActivationModal from "./components/ActivationModal.vue";

// Trial State
const trialStatus = ref<TrialStatus | null>(null);

// State
const selectedFile = ref<string | null>(null);
const machineID = ref("");
const showActivationModal = ref(false);
const selectedFields = ref<string[]>([]);

const fieldLabels = ref<{ [key: string]: string }>({});
const selectedFormat = ref<"xlsx" | "csv" | "json">("xlsx");
const outputPath = ref<string>("");

const fileName = computed(() => {
  if (!selectedFile.value) return "";
  return selectedFile.value.split(/[\\/]/).pop() || "";
});

const isLoading = ref(false);
const result = ref<ExtractResult | null>(null);
const previewRecords = ref<Record[]>([]);
const showPreview = ref(false);
// 预览时的字段快照：字段选择变化后预览数据即视为过期，
// 导出时回退到重新抽取而不是导出与所选字段不一致的旧数据。
const previewFieldsSnapshot = ref<string[]>([]);

const { notification, showNotification } = useNotification();
const {
  loadingText,
  progressPercent,
  isCancelling,
  etaLabel,
  resetProgress,
  cancelExtraction,
} = useExtractionProgress(isLoading);

// Actions
async function fetchTrialStatus() {
  try {
    const status = await api.service.getTrialStatus();
    trialStatus.value = status;
    // 同时获取机器码
    const mid = await api.service.getMachineID();
    machineID.value = mid;
  } catch (e) {
    console.error("Failed to fetch trial status:", e);
  }
}

async function handleActivate(key: string) {
  if (!key) return;

  loadingText.value = "正在验证授权...";
  isLoading.value = true;
  resetProgress();

  // 5. 强制延迟 500ms 确保 Loading 动画渲染出来，避免被系统弹窗（如权限请求）打断渲染
  await new Promise(resolve => setTimeout(resolve, 500));

  try {
    const success = await api.service.activate(key);

    // Stop loading BEFORE showing success to prevent overlay conflict
    isLoading.value = false;

    if (success) {
      showNotification("激活成功！感谢使用专业版", "success");
      showActivationModal.value = false;
      await fetchTrialStatus(); // 刷新 UI 状态
    } else {
      // 关键修复：增加失败反馈
      showNotification("授权码无效，请检查后重试", "error");
    }
  } catch (e) {
    isLoading.value = false;
    showNotification(String(e), "error");
  } finally {
    // Redundant safety check
    if (isLoading.value) isLoading.value = false;
    loadingText.value = "";
  }
}

onMounted(() => {
  fetchTrialStatus();
});

const operationState = computed(() => {
  if (isLoading.value) return isCancelling.value ? "正在取消" : "处理中";
  if (result.value?.success) return "已完成";
  if (result.value && !result.value.success) return "需处理";
  if (showPreview.value) return "已预览";
  if (selectedFile.value) return "待导出";
  return "待选择文件";
});

function handleFileUpdate(file: string) {
  selectedFile.value = file;
  // Reset state when file changes
  outputPath.value = "";
  selectedFields.value = []; // Clear previous selection
  result.value = null;
  previewRecords.value = [];
  showPreview.value = false;
  previewFieldsSnapshot.value = [];
}

async function handlePreview() {
  if (!selectedFile.value) return;

  isLoading.value = true;
  resetProgress();
  try {
    const res = await api.service.previewData(
      selectedFile.value,
      selectedFields.value,
    );
    if (res.success && res.records) {
      previewRecords.value = res.records;
      fieldLabels.value = res.fieldLabels || {};
      showPreview.value = true;
      previewFieldsSnapshot.value = [...selectedFields.value];
    } else if (isCancelledError(res.errorMessage)) {
      showNotification("已取消预览", "info");
    } else if (res.errorMessage) {
      showNotification(friendlyErrorMessage(res.errorMessage), "error");
    }
  } catch (e) {
    console.error("Preview failed:", e);
    showNotification("预览失败: " + (e as Error).message, "error");
  } finally {
    isLoading.value = false;
  }
}

// 预览数据与当前字段选择一致时，导出应基于（可能被用户编辑过的）预览数据，
// 而不是重新抽取——否则预览表格中的人工修正会被静默丢弃。
const canExportPreview = computed(
  () =>
    showPreview.value &&
    previewRecords.value.length > 0 &&
    sameFields(previewFieldsSnapshot.value, selectedFields.value),
);

function sameFields(a: string[], b: string[]) {
  return a.length === b.length && a.every((v) => b.includes(v));
}

async function handleExtract() {
  if (!selectedFile.value) return;

  isLoading.value = true;
  resetProgress();
  result.value = null;

  try {
    const defaultExt = selectedFormat.value;
    const baseName = fileName.value.includes('.')
      ? fileName.value.substring(0, fileName.value.lastIndexOf('.'))
      : fileName.value;
    const defaultName = `提取结果_${baseName}.${defaultExt}`;

    let finalOutputPath = outputPath.value;

    if (!finalOutputPath) {
      try {
        finalOutputPath = await api.service.selectOutputPath(defaultName);
        if (finalOutputPath) {
          outputPath.value = finalOutputPath;
        } else {
          isLoading.value = false;
          return;
        }
      } catch (e) {
        console.error("Select output path failed:", e);
        showNotification("选择保存路径失败", "error");
        isLoading.value = false;
        return;
      }
    }

    const res = canExportPreview.value
      ? await api.service.exportData(previewRecords.value, finalOutputPath)
      : await api.service.extractToPath(
          selectedFile.value,
          finalOutputPath,
          selectedFields.value,
        );

    if (res.success) {
      result.value = res as ExtractResult;
      showNotification(`提取成功！共 ${res.recordCount} 条记录`, "success");
    } else if (isCancelledError(res.errorMessage)) {
      showNotification("已取消提取", "info");
    } else {
      const message = friendlyErrorMessage(res.errorMessage);
      result.value = {
        success: false,
        recordCount: 0,
        outputPath: "",
        errorMessage: message,
      };
      showNotification(message, "error");
    }
  } catch (e) {
    console.error("Extraction failed:", e);
    result.value = {
      success: false,
      recordCount: 0,
      outputPath: "",
      errorMessage: (e as Error).message,
    };
    showNotification("提取过程发生错误", "error");
  } finally {
    isLoading.value = false;
  }
}

async function handleOpenFile(path: string) {
  try {
    await api.service.openFile(path);
  } catch (e) {
    console.error("Open file failed:", e);
    showNotification("打开文件失败", "error");
  }
}

function handleFieldsChange(fields: string[]) {
  selectedFields.value = fields;
}
</script>

<template>
  <div class="app-container">
    <!-- Notification Toast -->
    <Transition name="toast">
      <div v-if="notification" class="toast" :class="notification.type">
        <span class="toast-icon">
            <svg v-if="notification.type === 'error'" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>
            <svg v-else-if="notification.type === 'success'" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><path d="M22 4 12 14.01l-3-3"/></svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
        </span>
        <span class="toast-message">{{ notification.message }}</span>
      </div>
    </Transition>

    <!-- Loading Overlay -->
    <Transition name="fade">
      <div v-if="isLoading" class="loading-overlay">
        <div class="loading-card glass-panel">
          <div class="progress-container">
            <div class="progress-bar">
              <div class="progress-fill" :style="{ width: progressPercent + '%' }"></div>
            </div>
            <div class="progress-info">
              <span class="progress-label">当前进度</span>
              <span class="progress-value">{{ progressPercent }}%</span>
            </div>
            <div v-if="etaLabel" class="progress-eta">{{ etaLabel }}</div>
          </div>
          <div class="loading-content">
            <h3 class="loading-title">{{ isCancelling ? '正在取消...' : '正在处理中' }}</h3>
            <p class="loading-desc">{{ loadingText || (fileName.toLowerCase().endsWith('.pdf') ? '正在进行文档智能解析...' : '正在解析本地文档结构...') }}</p>
          </div>
          <button
            class="cancel-btn"
            type="button"
            :disabled="isCancelling"
            @click="cancelExtraction"
          >
            {{ isCancelling ? '取消中...' : '停止' }}
          </button>
        </div>
      </div>
    </Transition>

    <!-- Trial Banner + Active Badge -->
    <TrialBanner :trialStatus="trialStatus" @activate="showActivationModal = true" />

    <main class="main-content">
      <!-- Header -->
      <header class="header">
        <div class="header-brand">
          <div class="logo-container">
            <div class="logo-icon">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="22"
                height="22"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path
                  d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
                ></path>
                <polyline points="14 2 14 8 20 8"></polyline>
                <line x1="16" y1="13" x2="8" y2="13"></line>
                <line x1="16" y1="17" x2="8" y2="17"></line>
                <polyline points="10 9 9 9 8 9"></polyline>
              </svg>
            </div>
            <span class="logo-text font-heading">LegalExtractor</span>
          </div>
          <p class="subtitle">.docx / .pdf / 图片 法律文书结构化提取</p>
        </div>
        <div class="header-status">
          <span class="status-dot"></span>
          <span>{{ operationState }}</span>
        </div>
      </header>

      <!-- Main Action Area -->
      <div class="main-card glass-panel">
        <MainDropZone
          :selectedFile="selectedFile"
          :fileName="fileName"
          @update:selectedFile="handleFileUpdate"
          @notification="showNotification"
        />

        <ConfigPanel
          v-if="selectedFile"
          :selectedFile="selectedFile"
          :fileName="fileName"
          v-model:selectedFormat="selectedFormat"
          v-model:outputPath="outputPath"
          v-model:selectedFields="selectedFields"
          :isLoading="isLoading"
          :isDisabled="trialStatus?.isExpired ?? false"
          @preview="handlePreview"
          @extract="handleExtract"
        />
      </div>

      <!-- Result Section -->
      <ResultCard :result="result" @notification="showNotification" />

      <!-- Preview Table -->
      <Transition name="slide-up">
        <PreviewTable
          v-if="showPreview && previewRecords.length > 0"
          :records="previewRecords"
          :fieldLabels="fieldLabels"
        />
      </Transition>
    </main>

    <!-- Activation Modal -->
    <ActivationModal
      :show="showActivationModal"
      :machineID="machineID"
      @close="showActivationModal = false"
      @activate="handleActivate"
      @copied="showNotification('特征码已复制到剪贴板', 'success')"
    />
  </div>
</template>

<style scoped>
.app-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px;
  padding-top: 60px;
  position: relative;
  overflow-x: hidden;
  height: 100vh;
  overflow-y: auto;
  background: var(--bg-primary);
}

/* Main Content */
.main-content {
  width: 100%;
  max-width: 920px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  z-index: 1;
}

/* Header */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 4px;
}

.logo-container {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  color: var(--accent-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  background: rgba(34, 197, 94, 0.10);
  border: 1px solid rgba(34, 197, 94, 0.22);
  border-radius: 8px;
}

.logo-text {
  font-weight: 600;
  font-size: 1.15rem;
  letter-spacing: 0.2px;
  color: var(--text-primary);
}

.header-brand {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.subtitle {
  color: var(--text-secondary);
  font-size: 0.86rem;
  line-height: 1.3;
}

.header-status {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 0.8rem;
  padding: 8px 11px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.62);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success);
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.12);
}

/* Main Card */
.main-card {
  padding: 16px;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  transition:
    transform 0.3s ease,
    box-shadow 0.3s ease;
  background: rgba(15, 23, 42, 0.74);
}

.main-card:hover {
  box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.3);
}

.glass-panel {
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border: 1px solid rgba(148, 163, 184, 0.16);
}

/* Transitions */
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-20px) translateX(-50%);
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(20px);
}

/* Toast Styles */
.toast {
  position: fixed;
  top: 80px; /* 移至横幅下方，预留足够空间 */
  left: 50%;
  transform: translateX(-50%);
  background: rgba(15, 23, 42, 0.9);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  padding: 10px 24px;
  border-radius: var(--radius-full);
  display: flex;
  align-items: center;
  gap: 12px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
  z-index: 6000; /* Ensure it's above the modal (5000) */
  border: 1px solid rgba(255, 255, 255, 0.1);
  min-width: 320px;
}

.toast.success {
  border-color: rgba(var(--success-rgb), 0.3);
}

.toast.error {
  border-color: rgba(var(--error-rgb), 0.3);
}

.toast-icon {
  font-size: 1.2rem;
}

.toast-message {
  font-size: 0.95rem;
  color: var(--text-primary);
}

@media (max-width: 720px) {
  .app-container {
    padding: 16px;
    padding-top: 54px;
  }

  .main-content {
    gap: 12px;
  }

  .header {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }

  .header-status {
    align-self: flex-start;
    flex-shrink: 0;
  }

  .logo-text {
    font-size: 1rem;
  }

  .subtitle {
    font-size: 0.78rem;
  }

}

/* Loading Overlay */
.loading-overlay {
  position: fixed;
  inset: 0;
  background: rgba(2, 6, 23, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5500;
}

.loading-card {
  width: 90%;
  max-width: 400px;
  padding: 32px;
  border-radius: 24px;
  background: rgba(30, 41, 59, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  gap: 24px;
  text-align: center;
}

.progress-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.progress-bar {
  height: 8px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: var(--radius-full);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent-primary), #38bdf8);
  border-radius: var(--radius-full);
  transition: width 0.4s cubic-bezier(0.16, 1, 0.3, 1);
  box-shadow: 0 0 15px rgba(14, 165, 233, 0.4);
}

.progress-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.85rem;
  font-weight: 500;
}

.progress-label {
  color: var(--text-secondary);
}

.progress-value {
  color: var(--accent-primary);
  font-family: 'JetBrains Mono', monospace;
  font-size: 1rem;
}

.progress-eta {
  margin-top: 4px;
  font-size: 0.8rem;
  color: var(--text-secondary);
  text-align: right;
  font-family: 'JetBrains Mono', monospace;
}

.cancel-btn {
  align-self: center;
  margin-top: 12px;
  padding: 8px 20px;
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.35);
  color: #fca5a5;
  border-radius: var(--radius-full);
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.cancel-btn:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.22);
  border-color: rgba(239, 68, 68, 0.55);
  color: #fecaca;
}

.cancel-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.loading-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.loading-title {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 0.5px;
}

.loading-desc {
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.5;
  min-height: 1.4em; /* Prevent layout jump */
}
</style>
