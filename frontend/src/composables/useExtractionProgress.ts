import { computed, onMounted, ref, type Ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { api } from "../services";
import { formatEta, type ProgressSample } from "../utils/eta";

/**
 * 提取进度状态：监听后端 `extraction_progress` 事件，
 * 维护进度百分比 / ETA 采样 / 取消状态与加载文案。
 *
 * isLoading 由调用方持有（激活流程也复用同一个加载遮罩）。
 */
export function useExtractionProgress(isLoading: Ref<boolean>) {
  const loadingText = ref("");
  const progressPercent = ref(0);
  const progressTotal = ref(0);
  const progressSamples = ref<ProgressSample[]>([]);
  const isCancelling = ref(false);

  const etaLabel = computed(() => {
    if (!isLoading.value || isCancelling.value) return "";
    return formatEta(progressSamples.value, progressTotal.value);
  });

  /** 开始一次新的提取/预览前重置进度状态。 */
  function resetProgress() {
    isCancelling.value = false;
    progressPercent.value = 0;
    progressTotal.value = 0;
    progressSamples.value = [];
  }

  async function cancelExtraction() {
    if (isCancelling.value) return;
    isCancelling.value = true;
    try {
      await api.service.cancelExtraction();
    } catch (e) {
      console.error("Cancel failed:", e);
    }
  }

  onMounted(() => {
    EventsOn("extraction_progress", (data: { current: number; total: number; message: string }) => {
      progressPercent.value = Math.round((data.current / data.total) * 100);
      progressTotal.value = data.total;
      progressSamples.value.push({ t: Date.now(), current: data.current });
      // Keep at most the last 32 samples to bound memory in long extractions.
      if (progressSamples.value.length > 32) {
        progressSamples.value = progressSamples.value.slice(-32);
      }
      loadingText.value = data.message;
    });
  });

  return {
    loadingText,
    progressPercent,
    progressTotal,
    isCancelling,
    etaLabel,
    resetProgress,
    cancelExtraction,
  };
}
