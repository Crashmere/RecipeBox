<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api, write, notify } from "../api";
import type { Entry } from "../types";
import Icon from "../components/Icon.vue";
const entries = ref<Entry[]>([]),
  error = ref(""),
  loading = ref(true),
  busy = ref("");
async function load() {
  try {
    entries.value = await api<Entry[]>("records?trash=1");
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}
onMounted(load);
async function restore(e: Entry) {
  busy.value = e.id;
  try {
    await write(e.id, { revision: e.revision, action: "restore" });
    entries.value = entries.value.filter((x) => x.id !== e.id);
    notify("已恢复，可以继续编辑了");
  } catch (e) {
    notify((e as Error).message);
  } finally {
    busy.value = "";
  }
}
</script>
<template>
  <div class="page">
    <RouterLink to="/" class="back-link"
      ><Icon name="arrow" :size="18" />返回菜谱</RouterLink
    >
    <div class="page-title">
      <h1>回收站</h1>
      <p>不小心删掉的记录，在这里保留 30 天。</p>
    </div>
    <p v-if="error" class="notice error">{{ error }}</p>
    <div v-if="loading" class="empty">加载中…</div>
    <div v-else-if="!entries.length" class="empty">
      <Icon name="trash" :size="40" />
      <h3>回收站是空的</h3>
    </div>
    <div class="trash-list">
      <div v-for="e in entries" class="trash-item">
        <Icon :name="e.kind === 'recipe' ? 'book' : 'box'" />
        <div>
          <h3>{{ e.name }}</h3>
          <p>
            {{ e.kind === "recipe" ? "菜谱" : "食品" }} ·
            {{ e.deletedAt?.slice(0, 10) }} 移入
          </p>
        </div>
        <button class="btn secondary" :disabled="!!busy" @click="restore(e)">
          <Icon name="restore" />恢复
        </button>
      </div>
    </div>
  </div>
</template>
