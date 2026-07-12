<template>
  <div class="app-shell">
    <header class="topbar">
      <span class="brand">LEGAL·EXTRACTOR</span>
      <span class="brand-sub">法律文书结构化提取</span>
      <button
        class="trial-pill"
        :class="{ pro: trialStatus?.isActivated }"
        type="button"
        @click="!trialStatus?.isActivated && (showActivationModal = true)"
      >
        {{ trialPillText }}
      </button>
    </header>

    <main class="stage">
      <LandingView
        v-if="stage === 'landing'"
        @file="handleFile"
        @notify="showNotification"
      />

      <ParsingView
        v-else-if="stage === 'parsing'"
        :fileName="fileName"
        :percent="progressPercent"
        :message="loadingText"
        :etaLabel="etaLabel"
        :isCancelling="isCancelling"
        @cancel="cancelExtraction"
      />

      <div v-else class="desk">
        <DocPane
          :fileName="fileName"
          :kind="fileKind"
          :docData="docData"
          :currentPage="curPage"
          :totalPages="totalPages"
          :recordPages="recordPages"
          :recordsOnPage="recordsOnPage"
          @update:currentPage="curPage = $event"
          @locate="locateFromPage"
          @close="resetToLanding"
          @loaded="docNumPages = $event"
        />

        <section class="respane">
          <div class="res-head">
            <span class="res-title">结构化记录</span>
            <span class="res-meta">{{ records.length }} 条记录 · {{ selectedFields.length }} 字段</span>
            <FieldsPopover
              :options="fieldOptions"
              :selected="selectedFields"
              @update:selected="selectedFields = $event"
            />
          </div>

          <div class="res-scroll scroll-thin">
            <RecordCard
              v-for="(rec, i) in records"
              :key="i"
              :index="i"
              :record="rec"
              :fields="visibleFields"
              :open="openSet.has(i)"
              :edited="editedSet.has(i)"
              :flash="flashRec === i"
              @toggle="toggleRecord(i)"
              @locate="locateRecord(i)"
              @edit="(key, value) => editRecord(i, key, value)"
            />
          </div>

          <ExportBar
            :outputPath="outputPath"
            :format="format"
            :exporting="exporting"
            :done="exportDone"
            @update:format="setFormat"
            @choose-path="choosePath"
            @export="handleExport"
            @open-file="openExported"
            @dismiss-done="exportDone = null"
          />
        </section>
      </div>
    </main>

    <Transition name="toast">
      <div v-if="notification" class="toast" :class="notification.type" role="status" aria-live="polite">
        {{ notification.message }}
      </div>
    </Transition>

    <ActivationModal
      :show="showActivationModal"
      :machineID="machineID"
      @close="showActivationModal = false"
      @activate="handleActivate"
      @copied="showNotification('特征码已复制到剪贴板', 'success')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { api, friendlyErrorMessage, isCancelledError, type Record, type TrialStatus } from "./services";
import { useNotification } from "./composables/useNotification";
import { useExtractionProgress } from "./composables/useExtractionProgress";
import { FIELD_ORDER, fieldKind, type ExportFormat, type RecordField } from "./utils/fields";
import LandingView from "./components/LandingView.vue";
import ParsingView from "./components/ParsingView.vue";
import DocPane from "./components/DocPane.vue";
import RecordCard from "./components/RecordCard.vue";
import FieldsPopover from "./components/FieldsPopover.vue";
import ExportBar from "./components/ExportBar.vue";
import ActivationModal from "./components/ActivationModal.vue";

type Stage = "landing" | "parsing" | "desk";

// ── 全局状态 ──
const stage = ref<Stage>("landing");
const selectedFile = ref<string | null>(null);
const records = ref<Record[]>([]);
const fieldLabels = ref<{ [key: string]: string }>({});
const selectedFields = ref<string[]>([]);
const editedSet = ref<Set<number>>(new Set());
const openSet = ref<Set<number>>(new Set());
const flashRec = ref<number | null>(null);
const curPage = ref(1);
const outputPath = ref("");
const format = ref<ExportFormat>("xlsx");
const exporting = ref(false);
const exportDone = ref<{ path: string; count: number; edited: boolean } | null>(null);
// 原文内容（base64）与 pdf.js 上报的真实总页数
const docData = ref<string | null>(null);
const docNumPages = ref<number | null>(null);

const isLoading = ref(false);
const { notification, showNotification } = useNotification();
const {
  loadingText,
  progressPercent,
  isCancelling,
  etaLabel,
  resetProgress,
  cancelExtraction,
} = useExtractionProgress(isLoading);

// ── 试用/授权 ──
const trialStatus = ref<TrialStatus | null>(null);
const machineID = ref("");
const showActivationModal = ref(false);

const trialPillText = computed(() => {
  const t = trialStatus.value;
  if (!t) return "";
  if (t.isActivated) return "专业授权版";
  if (t.isExpired) return "试用已结束 · 联系授权";
  return `试用剩余 ${t.days} 天`;
});

async function fetchTrialStatus() {
  try {
    trialStatus.value = await api.service.getTrialStatus();
    machineID.value = await api.service.getMachineID();
  } catch (e) {
    console.error("Failed to fetch trial status:", e);
  }
}

async function handleActivate(key: string) {
  if (!key) return;
  try {
    const success = await api.service.activate(key);
    if (success) {
      showNotification("激活成功！感谢使用专业版", "success");
      showActivationModal.value = false;
      await fetchTrialStatus();
    } else {
      showNotification("授权码无效，请检查后重试", "error");
    }
  } catch (e) {
    showNotification(String(e), "error");
  }
}

// ── 派生数据 ──
const fileName = computed(() => {
  if (!selectedFile.value) return "";
  return selectedFile.value.split(/[\\/]/).pop() || "";
});

const fileKind = computed<"pdf" | "image" | "docx">(() => {
  const n = fileName.value.toLowerCase();
  if (n.endsWith(".pdf")) return "pdf";
  if (n.endsWith(".jpg") || n.endsWith(".jpeg") || n.endsWith(".png")) return "image";
  return "docx";
});

const fieldOptions = computed(() =>
  FIELD_ORDER.filter((k) => k in fieldLabels.value).map((k) => ({
    key: k,
    label: fieldLabels.value[k],
  })),
);

const visibleFields = computed<RecordField[]>(() =>
  FIELD_ORDER.filter((k) => selectedFields.value.includes(k)).map((k) => ({
    key: k,
    label: fieldLabels.value[k] || k,
    kind: fieldKind(k),
  })),
);

const recordPages = computed(() => {
  const pages = new Set<number>();
  for (const r of records.value) {
    const p = Number(r.page);
    if (p >= 1) pages.add(p);
  }
  return [...pages].sort((a, b) => a - b);
});

// 后端仅返回记录页码；PDF 由 pdf.js 上报真实总页数，其余取已知记录页最大值。
const totalPages = computed(() => Math.max(docNumPages.value || 1, ...recordPages.value, 1));

const recordsOnPage = computed(() =>
  records.value
    .map((r, index) => ({ index, page: Number(r.page) }))
    .filter((r) => r.page === curPage.value)
    .map((r) => ({ index: r.index, label: "REC " + String(r.index + 1).padStart(2, "0") })),
);

// ── 主流程：选择 → 解析（自动预览）→ 取证台 ──
async function handleFile(path: string) {
  selectedFile.value = path;
  records.value = [];
  editedSet.value = new Set();
  exportDone.value = null;
  outputPath.value = "";
  docLoadToken += 1;
  docData.value = null;
  docNumPages.value = null;
  stage.value = "parsing";
  isLoading.value = true;
  resetProgress();

  try {
    const res = await api.service.previewData(path, []);
    if (res.success && res.records && res.records.length > 0) {
      records.value = res.records;
      fieldLabels.value = res.fieldLabels || {};
      selectedFields.value = FIELD_ORDER.filter((k) => k in (res.fieldLabels || {}));
      openSet.value = new Set([0]);
      curPage.value = Number(res.records[0].page) || 1;
      stage.value = "desk";
      showNotification(`解析完成：提取到 ${res.records.length} 条记录`, "success");
      void loadDocForViewer();
    } else {
      stage.value = "landing";
      if (isCancelledError(res.errorMessage)) {
        showNotification("已取消解析", "info");
      } else {
        showNotification(friendlyErrorMessage(res.errorMessage), "error");
      }
    }
  } catch (e) {
    console.error("Preview failed:", e);
    stage.value = "landing";
    showNotification("解析失败: " + (e as Error).message, "error");
  } finally {
    isLoading.value = false;
  }
}

// 异步加载原文内容供左栏渲染；失败不阻断流程（DocPane 回退占位页）。
// docLoadToken 使换文件后仍在途的旧响应作废，防止旧文书内容覆盖 docData。
let docLoadToken = 0;

async function loadDocForViewer() {
  if (fileKind.value === "docx") return;
  const token = ++docLoadToken;
  try {
    const data = await api.service.readCurrentDocument();
    if (token === docLoadToken) {
      docData.value = data;
    }
  } catch (e) {
    console.error("读取原文失败:", e);
  }
}

function resetToLanding() {
  stage.value = "landing";
  selectedFile.value = null;
  records.value = [];
  exportDone.value = null;
  docLoadToken += 1;
  docData.value = null;
  docNumPages.value = null;
}

// ── 记录交互 ──
function toggleRecord(i: number) {
  const next = new Set(openSet.value);
  if (next.has(i)) next.delete(i);
  else next.add(i);
  openSet.value = next;
}

function editRecord(i: number, key: string, value: string) {
  records.value[i][key] = value;
  const next = new Set(editedSet.value);
  next.add(i);
  editedSet.value = next;
}

let flashTimer: ReturnType<typeof setTimeout> | null = null;

function locateRecord(i: number) {
  const page = Number(records.value[i].page);
  if (page >= 1) curPage.value = page;
}

function locateFromPage(i: number) {
  const next = new Set(openSet.value);
  next.add(i);
  openSet.value = next;
  flashRec.value = i;
  if (flashTimer) clearTimeout(flashTimer);
  flashTimer = setTimeout(() => {
    flashRec.value = null;
  }, 1200);
}

// ── 导出 ──
function defaultExportName() {
  const base = fileName.value.includes(".")
    ? fileName.value.substring(0, fileName.value.lastIndexOf("."))
    : fileName.value;
  return `提取结果_${base}.${format.value}`;
}

function setFormat(f: ExportFormat) {
  format.value = f;
  if (outputPath.value) {
    outputPath.value = outputPath.value.replace(/\.(xlsx|csv|json)$/i, "." + f);
  }
}

async function choosePath(): Promise<string> {
  try {
    const path = await api.service.selectOutputPath(defaultExportName());
    if (path) {
      outputPath.value = path;
      const m = path.toLowerCase().match(/\.(xlsx|csv|json)$/);
      if (m) format.value = m[1] as ExportFormat;
    }
    return path || "";
  } catch (e) {
    console.error("Select output path failed:", e);
    showNotification("选择保存路径失败", "error");
    return "";
  }
}

// 导出载荷：仅保留勾选字段 + 页码元信息，人工修正随记录一起导出
function exportPayload(): Record[] {
  const keep = new Set([...selectedFields.value, "page"]);
  return records.value.map((r) => {
    const out: Record = {};
    for (const k of Object.keys(r)) {
      if (keep.has(k)) out[k] = r[k];
    }
    return out;
  });
}

async function handleExport() {
  let path = outputPath.value;
  if (!path) {
    path = await choosePath();
    if (!path) return;
  }

  exporting.value = true;
  exportDone.value = null;
  try {
    const res = await api.service.exportData(exportPayload(), path);
    if (res.success) {
      exportDone.value = {
        path: res.outputPath || path,
        count: res.recordCount,
        edited: editedSet.value.size > 0,
      };
    } else if (isCancelledError(res.errorMessage)) {
      showNotification("已取消导出", "info");
    } else {
      showNotification(friendlyErrorMessage(res.errorMessage), "error");
    }
  } catch (e) {
    console.error("Export failed:", e);
    showNotification("导出过程发生错误", "error");
  } finally {
    exporting.value = false;
  }
}

async function openExported() {
  if (!exportDone.value) return;
  try {
    await api.service.openFile(exportDone.value.path);
  } catch (e) {
    console.error("Open file failed:", e);
    showNotification("打开文件失败", "error");
  }
}

// ── 键盘：取证台 ←/→ 翻页（输入框内不劫持） ──
function onKeydown(e: KeyboardEvent) {
  if (stage.value !== "desk") return;
  const tag = (e.target as HTMLElement).tagName;
  if (tag === "INPUT" || tag === "TEXTAREA") return;
  if (e.key === "ArrowLeft" && curPage.value > 1) curPage.value -= 1;
  if (e.key === "ArrowRight" && curPage.value < totalPages.value) curPage.value += 1;
}

onMounted(() => {
  fetchTrialStatus();
  document.addEventListener("keydown", onKeydown);
});

onUnmounted(() => {
  document.removeEventListener("keydown", onKeydown);
  if (flashTimer) clearTimeout(flashTimer);
});
</script>

<style scoped>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
  position: relative;
  overflow: hidden;
}

/* 顶栏 */
.topbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  height: 42px;
  flex-shrink: 0;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--surface-border);
}
.brand {
  font-family: var(--font-mono);
  font-size: 12px;
  letter-spacing: 1px;
  color: var(--accent-soft);
}
.brand-sub {
  font-size: 11px;
  color: var(--text-muted);
}
.trial-pill {
  margin-left: auto;
  font-family: var(--font-mono);
  font-size: 10px;
  color: #86efac;
  background: transparent;
  border: 1px solid rgba(52, 216, 121, 0.4);
  padding: 3px 10px;
  border-radius: var(--radius-full);
  cursor: pointer;
  transition: all 0.15s;
}
.trial-pill:hover {
  background: rgba(52, 216, 121, 0.12);
}
.trial-pill.pro {
  color: var(--bg-primary);
  background: var(--accent-primary);
  border-color: var(--accent-primary);
  font-weight: 700;
  cursor: default;
}

/* 舞台 */
.stage {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* 取证台双栏 */
.desk {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 372px 1fr;
}
.respane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.res-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 13px 16px 10px;
}
.res-title {
  font-size: 13.5px;
  font-weight: 700;
}
.res-meta {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-muted);
}
.res-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 16px 10px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* Toast */
.toast {
  position: absolute;
  bottom: 76px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(10, 22, 16, 0.97);
  border: 1px solid var(--surface-border-strong);
  border-radius: var(--radius-full);
  padding: 9px 20px;
  font-size: 12px;
  z-index: 60;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
  max-width: 80%;
}
.toast.error {
  border-color: rgba(255, 107, 94, 0.5);
  color: #ffc9c4;
}
.toast.success {
  border-color: rgba(52, 216, 121, 0.5);
}
.toast.info {
  color: var(--text-secondary);
}
.toast-enter-active,
.toast-leave-active {
  transition: all 0.22s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(8px);
}
</style>
