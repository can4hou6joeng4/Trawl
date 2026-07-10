<script setup lang="ts">
import type { TrialStatus } from "../services";

defineProps<{
  trialStatus: TrialStatus | null;
}>();

const emit = defineEmits<{
  (e: "activate"): void;
}>();
</script>

<template>
  <!-- Trial Banner -->
  <div v-if="trialStatus && !trialStatus.isActivated" class="trial-banner" :class="{ 'expired': trialStatus.isExpired }" role="status" aria-live="polite">
    <div class="trial-container">
      <template v-if="!trialStatus.isExpired">
        <span class="trial-icon">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        </span>
        <span class="trial-text">试用期剩余：<strong>{{ trialStatus.days }}</strong> 天 <strong>{{ trialStatus.hours }}</strong> 小时</span>
        <button class="trial-cta-btn" @click="emit('activate')">获取正式版</button>
      </template>
      <template v-else>
        <span class="trial-icon">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>
        </span>
        <span class="trial-text">试用期已结束，核心功能已锁定。</span>
        <button class="trial-cta-btn urgent" @click="emit('activate')">联系授权</button>
      </template>
    </div>
  </div>

  <!-- Professional Active Badge -->
  <div v-if="trialStatus?.isActivated" class="active-badge-fixed">
    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
    <span>专业授权版</span>
  </div>
</template>

<style scoped>
/* Trial Banner 试用期横幅 */
.trial-banner {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 38px;
  background: rgba(15, 23, 42, 0.92);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(148, 163, 184, 0.16);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  transition: all 0.3s ease;
}

.trial-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  max-width: 800px;
  width: 100%;
}

.trial-banner.expired {
  background: linear-gradient(90deg, rgba(69, 10, 10, 0.95) 0%, rgba(153, 27, 27, 0.95) 100%);
  border-bottom-color: rgba(239, 68, 68, 0.3);
}

.trial-icon {
  display: flex;
  align-items: center;
  color: var(--warning);
}

.trial-banner.expired .trial-icon {
  color: var(--error);
}

.trial-text {
  font-size: 0.8rem;
  color: var(--text-primary);
  font-weight: 500;
  letter-spacing: 0.3px;
}

.trial-text strong {
  color: var(--warning);
  margin: 0 2px;
}

.trial-cta-btn {
  background: rgba(34, 197, 94, 0.12);
  border: 1px solid rgba(34, 197, 94, 0.3);
  color: white;
  padding: 4px 12px;
  border-radius: 7px;
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  margin-left: 8px;
}

.trial-cta-btn:hover {
  background: var(--accent-primary);
  border-color: var(--accent-primary);
  transform: scale(1.05);
}

.trial-cta-btn.urgent {
  background: var(--error);
  border-color: var(--error);
}

.trial-cta-btn.urgent:hover {
  background: #dc2626;
  box-shadow: 0 0 12px rgba(239, 68, 68, 0.4);
}

/* Professional Active Badge */
.active-badge-fixed {
  position: fixed;
  top: 12px;
  right: 20px;
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
  padding: 4px 12px;
  border-radius: var(--radius-full);
  font-size: 0.75rem;
  font-weight: 600;
  backdrop-filter: blur(10px);
  z-index: 1000;
}
</style>
