import { ref } from "vue";

export type NotificationType = "success" | "error" | "info";

export interface Notification {
  message: string;
  type: NotificationType;
}

/**
 * 全局 Toast 通知状态：同一时间只显示一条，
 * 新通知会重置上一条的自动消失计时器。
 */
export function useNotification(durationMs = 3000) {
  const notification = ref<Notification | null>(null);
  let toastTimer: ReturnType<typeof setTimeout> | null = null;

  function showNotification(message: string, type: NotificationType = "info") {
    if (toastTimer) {
      clearTimeout(toastTimer);
      toastTimer = null;
    }

    notification.value = { message, type };
    toastTimer = setTimeout(() => {
      notification.value = null;
    }, durationMs);
  }

  return { notification, showNotification };
}
