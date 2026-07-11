<script setup lang="ts">
import type { Record } from "../services";
import type { RecordField } from "../utils/fields";

const props = defineProps<{
  index: number;
  record: Record;
  fields: RecordField[];
  open: boolean;
  edited: boolean;
  flash: boolean;
}>();

const emit = defineEmits<{
  (e: "toggle"): void;
  (e: "locate"): void;
  (e: "edit", key: string, value: string): void;
}>();

function recId(i: number) {
  return "REC " + String(i + 1).padStart(2, "0");
}

function onInput(key: string, ev: Event) {
  emit("edit", key, (ev.target as HTMLInputElement | HTMLTextAreaElement).value);
}

function summary() {
  const first = props.fields.find((f) => (props.record[f.key] || "").trim() !== "");
  return first ? props.record[first.key] : "";
}
</script>

<template>
  <div class="record" :class="{ flash }">
    <div class="rec-head" @click="emit('toggle')">
      <span class="rid">{{ recId(index) }}<template v-if="record.page"> · P.{{ record.page }}</template></span>
      <span v-if="!open" class="sum">{{ summary() }}</span>
      <span v-if="edited" class="edited">● 已修正</span>
      <button class="loc" type="button" @click.stop="emit('locate')">↩ 定位原文</button>
      <span class="caret" :class="{ open }">▾</span>
    </div>
    <template v-if="open">
      <div v-for="f in fields" :key="f.key" class="frow">
        <span class="k">{{ f.label }}</span>
        <textarea
          v-if="f.kind === 'long'"
          rows="2"
          :value="record[f.key]"
          :aria-label="f.label"
          spellcheck="false"
          @input="onInput(f.key, $event)"
        ></textarea>
        <input
          v-else
          :class="{ num: f.kind === 'num' }"
          :value="record[f.key]"
          :aria-label="f.label"
          spellcheck="false"
          @input="onInput(f.key, $event)"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.record {
  border: 1px solid rgba(84, 140, 105, 0.28);
  border-radius: 10px;
  background: var(--surface-panel);
  overflow: hidden;
  flex-shrink: 0;
  transition: border-color 0.2s;
}
.record.flash {
  animation: recflash 0.55s ease-in-out 2;
}
@keyframes recflash {
  50% {
    border-color: var(--accent-primary);
    box-shadow: 0 0 0 3px rgba(52, 216, 121, 0.18);
  }
}
.rec-head {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 13px;
  background: rgba(24, 178, 93, 0.08);
  border-bottom: 1px solid var(--surface-border);
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--accent-primary);
  letter-spacing: 1px;
  cursor: pointer;
}
.rec-head .sum {
  color: var(--text-secondary);
  font-family: var(--font-body);
  letter-spacing: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rec-head .edited {
  color: var(--warning);
  font-size: 9px;
  flex-shrink: 0;
}
.rec-head .loc {
  margin-left: auto;
  color: var(--text-muted);
  background: none;
  border: none;
  font-family: inherit;
  font-size: inherit;
  letter-spacing: inherit;
  cursor: pointer;
  transition: color 0.13s;
  flex-shrink: 0;
}
.rec-head .loc:hover {
  color: var(--accent-primary);
}
.rec-head .caret {
  color: var(--text-muted);
  font-size: 9px;
  transition: transform 0.2s;
  flex-shrink: 0;
}
.rec-head .caret.open {
  transform: rotate(180deg);
}
.frow {
  display: grid;
  grid-template-columns: 86px 1fr;
  gap: 12px;
  padding: 8px 13px;
  border-bottom: 1px solid rgba(84, 140, 105, 0.1);
  font-size: 12px;
  line-height: 1.6;
}
.frow:last-child {
  border-bottom: none;
}
.frow .k {
  color: var(--text-muted);
  font-size: 11px;
  padding-top: 3px;
}
.frow input,
.frow textarea {
  width: 100%;
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.6;
  border-bottom: 1px dashed transparent;
  padding: 2px 0;
  resize: none;
  transition: border-color 0.15s, background 0.15s;
  font-family: var(--font-body);
}
.frow input.num {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--accent-soft);
  letter-spacing: 0.3px;
}
.frow input:hover,
.frow textarea:hover {
  border-bottom-color: rgba(52, 216, 121, 0.35);
}
.frow input:focus,
.frow textarea:focus {
  border-bottom-color: var(--accent-primary);
  background: rgba(24, 178, 93, 0.05);
}
</style>
