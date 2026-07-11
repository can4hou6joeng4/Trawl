<script setup lang="ts">
defineProps<{
  fileName: string;
  percent: number;
  message: string;
  etaLabel: string;
  isCancelling: boolean;
}>();

const emit = defineEmits<{
  (e: "cancel"): void;
}>();

const mockLines = [86, 70, 82, 38, 90, 84, 33, 88];
</script>

<template>
  <div class="parsing">
    <div class="parse-file">
      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
      <span>{{ fileName }}</span>
    </div>
    <div class="parse-page">
      <div class="pp-line t"></div>
      <div v-for="(w, i) in mockLines" :key="i" class="pp-line" :style="{ width: w + '%' }"></div>
      <div class="scanbeam"></div>
    </div>
    <div class="parse-meter">
      <div class="parse-track">
        <div class="parse-fill" :style="{ width: percent + '%' }"></div>
      </div>
      <div class="parse-stats">
        <span class="pct">{{ percent }}%</span>
        <span v-if="etaLabel">{{ etaLabel }}</span>
      </div>
    </div>
    <div class="parse-msg">{{ message || "正在解析文书结构..." }}</div>
    <button class="btn-cancel" type="button" :disabled="isCancelling" @click="emit('cancel')">
      {{ isCancelling ? "取消中..." : "停止解析" }}
    </button>
  </div>
</template>

<style scoped>
.parsing {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 24px;
}
.parse-file {
  display: flex;
  align-items: center;
  gap: 10px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--accent-soft);
}
.parse-page {
  width: 190px;
  height: 240px;
  background: #f5f2ea;
  border-radius: 5px;
  position: relative;
  overflow: hidden;
  box-shadow: 0 12px 44px rgba(0, 0, 0, 0.55);
  padding: 20px 0 0;
}
.pp-line {
  height: 5px;
  border-radius: 2px;
  background: #c9c3b4;
  margin: 0 18px 8px;
}
.pp-line.t {
  background: #8f887a;
  height: 7px;
  width: 45%;
  margin: 0 auto 16px;
}
.scanbeam {
  position: absolute;
  left: 0;
  right: 0;
  height: 46px;
  background: linear-gradient(180deg, rgba(52, 216, 121, 0), rgba(52, 216, 121, 0.28) 50%, rgba(52, 216, 121, 0));
  border-top: 1px solid rgba(52, 216, 121, 0.7);
  animation: beam 1.15s ease-in-out infinite alternate;
}
@keyframes beam {
  from { top: -8%; }
  to { top: 88%; }
}
@media (prefers-reduced-motion: reduce) {
  .scanbeam { animation: none; top: 40%; }
}
.parse-meter {
  width: 380px;
  max-width: 70%;
  display: flex;
  flex-direction: column;
  gap: 9px;
}
.parse-track {
  height: 4px;
  border-radius: 2px;
  background: rgba(84, 140, 105, 0.2);
  overflow: hidden;
}
.parse-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent-deep), var(--accent-primary));
  border-radius: 2px;
  transition: width 0.25s linear;
}
.parse-stats {
  display: flex;
  justify-content: space-between;
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--text-muted);
}
.parse-stats .pct {
  color: var(--accent-soft);
}
.parse-msg {
  font-size: 12px;
  color: var(--text-secondary);
  min-height: 1.5em;
  text-align: center;
}
.btn-cancel {
  font-family: var(--font-mono);
  font-size: 11px;
  color: #fca5a5;
  border: 1px solid rgba(255, 107, 94, 0.35);
  background: transparent;
  border-radius: 999px;
  padding: 7px 20px;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-cancel:hover:not(:disabled) {
  background: rgba(255, 107, 94, 0.12);
}
.btn-cancel:disabled {
  opacity: 0.5;
  cursor: default;
}
</style>
