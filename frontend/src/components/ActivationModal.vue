<script setup lang="ts">
import { computed, ref, watch } from "vue";

const props = defineProps<{
  show: boolean;
  machineID: string;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "activate", key: string): void;
  (e: "copied"): void;
}>();

const licenseKey = ref("");

// 关闭弹窗时清空输入
watch(
  () => props.show,
  (newVal) => {
    if (!newVal) {
      licenseKey.value = "";
    }
  },
);

// Simple format check: XXXX-XXXX-XXXX-XXXX (approx 19 chars)
// We relax it slightly to allow loose input but prevent empty/short nonsense
const isValidLicense = computed(() => licenseKey.value.trim().length >= 16);

function copyMachineID() {
  navigator.clipboard.writeText(props.machineID);
  emit("copied");
}

function submit() {
  const key = licenseKey.value.trim();
  if (!key) return;
  emit("activate", key);
}
</script>

<template>
  <Transition name="fade">
    <div v-if="show" class="modal-overlay">
      <div class="activation-card">
        <div class="modal-header">
          <h2>软件激活</h2>
          <button class="close-btn" type="button" @click="emit('close')">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
          </button>
        </div>

        <div class="modal-body">
          <div class="info-section">
            <label>设备特征码 · 点击复制</label>
            <div class="machine-id-box" @click="copyMachineID">
              <code>{{ machineID }}</code>
              <svg class="copy-icon" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
            </div>
            <p class="helper-text">请将上方特征码发送给开发者以获取授权码</p>
          </div>

          <div class="input-section">
            <label>授权码</label>
            <input
              v-model="licenseKey"
              type="text"
              placeholder="XXXX-XXXX-XXXX-XXXX"
              class="license-input"
              spellcheck="false"
              @keyup.enter="submit"
            />
          </div>

          <button class="btn btn-primary" type="button" :disabled="!isValidLicense" @click="submit">
            激活专业版
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(4, 10, 7, 0.78);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5000;
}

.activation-card {
  width: 400px;
  max-width: 90%;
  background: #0c1811;
  border: 1px solid var(--surface-border-strong);
  border-radius: 14px;
  padding: 26px;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.7);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.modal-header h2 {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  padding: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  border-radius: 6px;
  transition: all 0.15s;
}

.close-btn:hover {
  color: var(--error);
  background: rgba(255, 107, 94, 0.1);
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.info-section label,
.input-section label {
  display: block;
  font-family: var(--font-mono);
  font-size: 10px;
  letter-spacing: 1px;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.machine-id-box {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  background: rgba(4, 23, 12, 0.6);
  border: 1px solid var(--surface-border-strong);
  border-radius: 9px;
  padding: 13px;
  cursor: pointer;
  transition: border-color 0.15s;
}

.machine-id-box:hover {
  border-color: var(--accent-primary);
}

.machine-id-box code {
  font-family: var(--font-mono);
  font-size: 17px;
  letter-spacing: 3px;
  color: var(--accent-primary);
}

.machine-id-box .copy-icon {
  color: var(--text-muted);
}

.helper-text {
  font-size: 10.5px;
  color: var(--text-muted);
  margin-top: 8px;
}

.license-input {
  width: 100%;
  background: rgba(4, 23, 12, 0.6);
  border: 1px solid var(--surface-border-strong);
  border-radius: 9px;
  padding: 12px;
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 13px;
  letter-spacing: 2px;
  text-align: center;
  outline: none;
  transition: border-color 0.15s;
}

.license-input:focus {
  border-color: var(--accent-primary);
}

.btn-primary {
  width: 100%;
  background: var(--accent-primary);
  color: #04170c;
  font-weight: 800;
  font-size: 13px;
  padding: 12px;
  border: none;
  border-radius: 9px;
  cursor: pointer;
  transition: opacity 0.15s;
}

.btn-primary:disabled {
  opacity: 0.35;
  cursor: default;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
