<script setup lang="ts">
const props = defineProps<{
  fileName: string;
  currentPage: number;
  totalPages: number;
  /** 产生了记录的页码集合 */
  recordPages: number[];
  /** 当前页上的记录（用于页面上的定位入口） */
  recordsOnPage: { index: number; label: string }[];
}>();

const emit = defineEmits<{
  (e: "update:currentPage", page: number): void;
  (e: "locate", recordIndex: number): void;
  (e: "close"): void;
}>();

const mockLines = [
  { w: "88%" }, { w: "72%" }, { w: "84%", mb: 22 },
  { w: "40%" }, { w: "91%" }, { w: "86%", mb: 22 },
  { w: "34%" }, { w: "89%" }, { w: "93%" }, { w: "58%" },
];

function go(delta: number) {
  const next = props.currentPage + delta;
  if (next >= 1 && next <= props.totalPages) {
    emit("update:currentPage", next);
  }
}
</script>

<template>
  <div class="docpane">
    <div class="doc-name">
      <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
      <span class="p" :title="fileName">{{ fileName }}</span>
      <button class="back" type="button" @click="emit('close')">✕ 关闭</button>
    </div>

    <div class="pagecard">
      <div class="pl t"></div>
      <div
        v-for="(l, i) in mockLines"
        :key="i"
        class="pl"
        :style="{ width: l.w, marginBottom: l.mb ? l.mb + 'px' : undefined }"
      ></div>
      <div v-if="recordsOnPage.length > 0" class="page-recs">
        <button
          v-for="r in recordsOnPage"
          :key="r.index"
          class="page-rec"
          type="button"
          :title="'定位到 ' + r.label"
          @click="emit('locate', r.index)"
        >{{ r.label }}</button>
      </div>
      <div v-else class="page-empty">本页无匹配记录</div>
    </div>

    <div class="pagenav">
      <button class="nav" type="button" :disabled="currentPage <= 1" @click="go(-1)">◂</button>
      <span class="cur">第 {{ currentPage }} / {{ totalPages }} 页</span>
      <button class="nav" type="button" :disabled="currentPage >= totalPages" @click="go(1)">▸</button>
      <span class="dotrow">
        <button
          v-for="i in totalPages"
          :key="i"
          class="pdot"
          type="button"
          :class="{ has: recordPages.includes(i), cur: currentPage === i }"
          :title="'第 ' + i + ' 页'"
          @click="emit('update:currentPage', i)"
        ></button>
      </span>
    </div>
  </div>
</template>

<style scoped>
.docpane {
  border-right: 1px solid var(--surface-border);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}
.doc-name {
  font-size: 11.5px;
  color: var(--accent-soft);
  font-family: var(--font-mono);
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}
.doc-name .p {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.doc-name .back {
  margin-left: auto;
  color: var(--text-muted);
  font-size: 10px;
  flex-shrink: 0;
  background: none;
  border: none;
  cursor: pointer;
  font-family: inherit;
}
.doc-name .back:hover {
  color: var(--error);
}
.pagecard {
  flex: 1;
  min-height: 0;
  background: #f5f2ea;
  border-radius: 6px;
  position: relative;
  overflow: hidden;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
  padding: 26px 24px;
}
.pl {
  height: 7px;
  border-radius: 2px;
  background: #c9c3b4;
  margin-bottom: 9px;
}
.pl.t {
  background: #8f887a;
  height: 10px;
  width: 46%;
  margin: 0 auto 20px;
}
.page-recs {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.page-rec {
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: #04170c;
  background: var(--accent-deep);
  border: none;
  border-radius: 3px;
  padding: 3px 8px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(24, 178, 93, 0.4);
  transition: transform 0.12s;
}
.page-rec:hover {
  transform: scale(1.06);
}
.page-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding-bottom: 18px;
  font-family: var(--font-mono);
  font-size: 9.5px;
  color: #a49d8d;
  letter-spacing: 1px;
  pointer-events: none;
}
.pagenav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--accent-soft);
}
.pagenav .cur {
  white-space: nowrap;
}
.pagenav .nav {
  width: 24px;
  height: 24px;
  border: 1px solid var(--surface-border-strong);
  background: transparent;
  border-radius: 5px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.13s;
  font-family: inherit;
}
.pagenav .nav:hover:not(:disabled) {
  color: var(--accent-primary);
  border-color: var(--accent-primary);
}
.pagenav .nav:disabled {
  opacity: 0.35;
  cursor: default;
}
.pagenav .dotrow {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  max-width: 120px;
}
.pagenav .pdot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(84, 140, 105, 0.35);
  border: none;
  padding: 0;
  cursor: pointer;
}
.pagenav .pdot.has {
  background: var(--accent-deep);
}
.pagenav .pdot.cur {
  outline: 1.5px solid var(--accent-primary);
  outline-offset: 1.5px;
  background: var(--accent-primary);
}
</style>
