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
      <div class="activation-card glass-panel">
        <div class="modal-header">
          <h2 class="font-heading">软件激活中心</h2>
          <button class="close-btn" @click="emit('close')">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
          </button>
        </div>

        <div class="modal-body">
          <div class="info-section">
            <label>您的设备特征码</label>
            <div class="machine-id-box" @click="copyMachineID">
              <code>{{ machineID }}</code>
              <svg class="copy-icon" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
            </div>
            <p class="helper-text">请将上方代码发送给开发者以获取授权码</p>
          </div>

          <div class="input-section">
            <label>输入授权码</label>
            <input
              v-model="licenseKey"
              type="text"
              placeholder="XXXX-XXXX-XXXX-XXXX"
              class="license-input"
              @keyup.enter="submit"
            />
          </div>

          <button class="btn btn-primary btn-glow full-width" @click="submit" :disabled="!isValidLicense">
            立即激活专业版
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
  background: rgba(2, 6, 23, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 5000;
}

/* 原 App.vue 作用域内的 glass-panel 增强（拆分后子组件内部元素
   无法继承父作用域样式，此处保持视觉一致） */
.glass-panel {
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border: 1px solid rgba(148, 163, 184, 0.16);
}

.activation-card {
  width: 90%;
  max-width: 440px;
  padding: 40px;
  border-radius: 24px;
  background: rgba(30, 41, 59, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  position: relative;
  animation: modalIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes modalIn {
  from { opacity: 0; transform: scale(0.9) translateY(20px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.modal-header h2 {
  font-size: 1.6rem;
  font-weight: 700;
  color: var(--text-primary);
}

.close-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-muted);
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: rgba(239, 68, 68, 0.2);
  color: #f87171;
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.info-section label, .input-section label {
  display: block;
  font-size: 0.9rem;
  color: var(--text-secondary);
  margin-bottom: 10px;
  font-weight: 500;
}

.machine-id-box {
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(14, 165, 233, 0.2);
  padding: 14px 18px;
  border-radius: 12px;
  display: flex;
  justify-content: center; /* 3. Centered */
  position: relative;      /* For absolute positioning of icon */
  align-items: center;
  cursor: pointer;
  transition: all 0.3s ease;
}

.machine-id-box:hover {
  border-color: var(--accent-primary);
  background: rgba(14, 165, 233, 0.08);
}

.machine-id-box code {
  font-family: 'JetBrains Mono', monospace;
  color: var(--accent-primary);
  font-size: 1.2rem;
  letter-spacing: 2px;
  font-weight: 600;
}

.machine-id-box .copy-icon {
  position: absolute;
  right: 18px;
  color: var(--text-secondary);
}

.license-input {
  width: 100%;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 14px 18px;
  border-radius: 12px;
  color: var(--text-primary);
  font-family: 'JetBrains Mono', monospace;
  font-size: 1.1rem;
  outline: none;
  transition: all 0.3s ease;
  text-align: center;
}

.license-input:focus {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 4px rgba(14, 165, 233, 0.15);
}

.full-width {
  width: 100%;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.1rem !important;
}
</style>
