<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";

const props = defineProps<{
  options: { key: string; label: string }[];
  selected: string[];
}>();

const emit = defineEmits<{
  (e: "update:selected", value: string[]): void;
}>();

const open = ref(false);
const rootEl = ref<HTMLElement | null>(null);

function toggle(key: string) {
  const has = props.selected.includes(key);
  if (has) {
    // 至少保留 1 个字段
    if (props.selected.length <= 1) return;
    emit("update:selected", props.selected.filter((k) => k !== key));
  } else {
    emit("update:selected", [...props.selected, key]);
  }
}

function onOutside(e: PointerEvent) {
  if (open.value && rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false;
  }
}

onMounted(() => document.addEventListener("pointerdown", onOutside, true));
onUnmounted(() => document.removeEventListener("pointerdown", onOutside, true));
</script>

<template>
  <div ref="rootEl" class="fields-root">
    <button class="fields-btn" type="button" @click="open = !open">
      ⚙ 字段 {{ selected.length }}/{{ options.length }}
    </button>
    <div v-if="open" class="fields-pop">
      <label v-for="o in options" :key="o.key" @click.prevent="toggle(o.key)">
        <span class="fcheck" :class="{ on: selected.includes(o.key) }">
          {{ selected.includes(o.key) ? "✓" : "" }}
        </span>
        {{ o.label }}
      </label>
      <div class="fields-hint">至少保留 1 个字段 · 仅影响显示与导出列</div>
    </div>
  </div>
</template>

<style scoped>
.fields-root {
  position: relative;
  margin-left: auto;
}
.fields-btn {
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--accent-soft);
  background: transparent;
  border: 1px solid var(--surface-border-strong);
  padding: 5px 11px;
  border-radius: 999px;
  cursor: pointer;
  transition: border-color 0.15s;
}
.fields-btn:hover {
  border-color: var(--accent-primary);
}
.fields-pop {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  background: #0d1a12;
  border: 1px solid var(--surface-border-strong);
  border-radius: 10px;
  padding: 8px;
  z-index: 30;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.6);
  min-width: 190px;
}
.fields-pop label {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 10px;
  border-radius: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  cursor: pointer;
}
.fields-pop label:hover {
  background: rgba(24, 178, 93, 0.08);
}
.fcheck {
  width: 15px;
  height: 15px;
  border: 1px solid var(--surface-border-strong);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--bg-primary);
  flex-shrink: 0;
  font-size: 10px;
}
.fcheck.on {
  background: var(--accent-primary);
  border-color: var(--accent-primary);
}
.fields-hint {
  font-size: 10px;
  color: var(--text-muted);
  padding: 6px 10px 3px;
  border-top: 1px solid var(--surface-border);
  margin-top: 5px;
}
</style>
