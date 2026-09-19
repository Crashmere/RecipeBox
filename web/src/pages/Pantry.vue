<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { api, media, write, notify } from "../api";
import { type Entry, expiryDays, expiryText } from "../types";
import Icon from "../components/Icon.vue";
import Modal from "../components/Modal.vue";
const operationError = ref("");
const entries = ref<Entry[]>([]),
  loading = ref(true),
  error = ref(""),
  q = ref(""),
  filter = ref("stock"),
  busy = ref(""),
  deleting = ref<Entry | null>(null);
async function load() {
  error.value = "";
  loading.value = true;
  try {
    entries.value = (await api<Entry[]>("records")).filter(
      (x) => x.kind === "pantry",
    );
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}
onMounted(load);
const expiring = computed(() =>
  entries.value.filter(
    (e) =>
      e.quantity > 0 &&
      expiryDays(e.expiryDate) !== null &&
      expiryDays(e.expiryDate)! >= 0 &&
      expiryDays(e.expiryDate)! <= 30,
  ),
);
const expired = computed(() =>
  entries.value.filter(
    (e) =>
      e.quantity > 0 &&
      expiryDays(e.expiryDate) !== null &&
      expiryDays(e.expiryDate)! < 0,
  ),
);
const items = computed(() =>
  entries.value
    .filter((e) => {
      const d = expiryDays(e.expiryDate);
      return (
        [e.name, e.category, e.location, e.notes]
          .join(" ")
          .toLowerCase()
          .includes(q.value.toLowerCase().trim()) &&
        (filter.value === "all" ||
          (filter.value === "stock" && e.quantity > 0) ||
          (filter.value === "empty" && !e.quantity) ||
          (filter.value === "soon" && e.quantity > 0 && d !== null && d <= 30))
      );
    })
    .sort(
      (a, b) =>
        (a.quantity === 0 ? 1 : 0) - (b.quantity === 0 ? 1 : 0) ||
        (a.expiryDate || "9999").localeCompare(b.expiryDate || "9999"),
    ),
);
async function action(e: Entry, action: string) {
  if (busy.value) return;
  busy.value = e.id;
  operationError.value = "";
  try {
    const saved = await write(e.id, { revision: e.revision, action });
    entries.value =
      action === "delete"
        ? entries.value.filter((x) => x.id !== e.id)
        : entries.value.map((x) => (x.id === e.id ? saved : x));
    notify(
      action === "consume"
        ? saved.quantity
          ? "已吃掉一" +
            (saved.unit || "份") +
            "，还剩 " +
            saved.quantity +
            (saved.unit || "份")
          : "这份食品吃完啦"
        : "已移入回收站",
    );
    deleting.value = null;
  } catch (e) {
    operationError.value = (e as Error).message;
    notify(operationError.value);
  } finally {
    busy.value = "";
  }
}
</script>
<template>
  <div class="page pantry-page">
    <div class="pantry-intro">
      <div>
        <div class="eyebrow">THE LITTLE PANTRY</div>
        <h1>厨房里的小储备</h1>
        <p>买过的记得吃，喜欢的记得补。</p>
      </div>
      <RouterLink to="/pantry/new" class="btn primary"
        ><Icon name="plus" />添加食品</RouterLink
      >
    </div>
    <div class="pantry-stats">
      <div>
        <Icon name="box" /><span
          >在库食品<strong
            >{{ entries.filter((x) => x.quantity > 0).length }}
            <small>种</small></strong
          ></span
        >
      </div>
      <button @click="filter = 'soon'">
        <Icon name="clock" /><span
          >30 天内到期<strong
            >{{ expiring.length }} <small>种</small></strong
          ></span
        ></button
      ><button :class="{ warning: expired.length }" @click="filter = 'soon'">
        <Icon name="leaf" /><span
          >已过期，请检查<strong
            >{{ expired.length }} <small>种</small></strong
          ></span
        >
      </button>
    </div>
    <div class="tools">
      <label class="search"
        ><Icon name="search" /><input
          v-model="q"
          placeholder="搜食品、分类或存放位置…"
          aria-label="搜索食品"
      /></label>
    </div>
    <div class="chip-row">
      <button
        v-for="f in [
          { id: 'stock', name: '还没吃完' },
          { id: 'soon', name: '优先吃这些' },
          { id: 'empty', name: '已经吃完' },
          { id: 'all', name: '全部食品' },
        ]"
        class="chip"
        :class="{ active: filter === f.id }"
        :aria-pressed="filter === f.id"
        @click="filter = f.id"
      >
        {{ f.name }}
      </button>
    </div>
    <div v-if="loading" class="empty">正在打开食品柜…</div>
    <div v-else-if="error" class="notice error">
      {{ error }}<button class="btn secondary" @click="load">重试</button>
    </div>
    <div v-else-if="!items.length" class="empty">
      <div class="empty-icon"><Icon name="box" :size="40" /></div>
      <h3>{{ entries.length ? "这一格，空空的" : "把厨房的小储备记下来" }}</h3>
      <p>泡面、挂面、罐头、零食… 记得它们放在哪里、什么时候吃。</p>
      <RouterLink to="/pantry/new" class="btn primary"
        ><Icon name="plus" />添加一份食品</RouterLink
      >
    </div>
    <div v-else class="pantry-list">
      <article v-for="e in items" :key="e.id" class="pantry-item">
        <RouterLink :to="'/edit/' + e.id" class="pantry-photo"
          ><img
            v-if="e.photoIds.length"
            :src="media(e.photoIds[0], true)"
            :alt="e.name"
            loading="lazy" /><Icon v-else name="box" :size="30"
        /></RouterLink>
        <div class="pantry-info">
          <div class="eyebrow">{{ e.category || "常备食品" }}</div>
          <RouterLink :to="'/edit/' + e.id"
            ><h3>{{ e.name }}</h3></RouterLink
          >
          <p>{{ e.location || "尚未记录存放位置" }}</p>
          <span
            class="expiry"
            :class="{
              soon:
                e.quantity > 0 &&
                expiryDays(e.expiryDate) !== null &&
                expiryDays(e.expiryDate)! <= 30,
              expired:
                e.quantity > 0 &&
                expiryDays(e.expiryDate) !== null &&
                expiryDays(e.expiryDate)! < 0,
            }"
            ><Icon name="clock" :size="14" />{{ expiryText(e) }}</span
          >
        </div>
        <div class="pantry-quantity">
          <strong>{{ e.quantity }}</strong
          ><span>{{ e.unit || "份" }}</span>
        </div>
        <div class="pantry-actions">
          <button
            class="btn secondary"
            v-if="e.quantity"
            :disabled="busy === e.id"
            @click="action(e, 'consume')"
          >
            吃掉一{{ e.unit || "份" }}</button
          ><RouterLink v-else :to="'/edit/' + e.id" class="btn secondary"
            >补货 / 修改</RouterLink
          >
          <div>
            <RouterLink
              :to="'/edit/' + e.id"
              class="icon-btn"
              :aria-label="'编辑 ' + e.name"
              ><Icon name="edit" :size="17" /></RouterLink
            ><button
              class="icon-btn"
              :aria-label="'删除 ' + e.name"
              @click="deleting = e"
            >
              <Icon name="trash" :size="17" />
            </button>
          </div>
        </div>
      </article>
    </div>
    <footer class="page-footer">
      到期日是贴心提醒，食用前也记得检查包装和保存状况。
    </footer>
    <Modal v-if="deleting" title="移走这份食品记录？" @close="deleting = null"
      ><p v-if="operationError" class="notice error" role="alert">
        {{ operationError }}
      </p>
      <p>「{{ deleting.name }}」会移入回收站，30 天内可以恢复。</p>
      <div class="modal-actions">
        <button class="btn secondary" @click="deleting = null">取消</button
        ><button
          class="btn danger-btn"
          :disabled="!!busy"
          @click="action(deleting, 'delete')"
        >
          移入回收站
        </button>
      </div></Modal
    >
  </div>
</template>
