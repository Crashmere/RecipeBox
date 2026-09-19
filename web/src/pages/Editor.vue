<script setup lang="ts">
import { ref, computed, onMounted, watch, onBeforeUnmount } from "vue";
import { useRoute, useRouter, onBeforeRouteLeave } from "vue-router";
import { api, write, stored, persist, forget, notify, ApiError } from "../api";
import {
  blank,
  type Entry,
  recipeCategories,
  pantryCategories,
} from "../types";
import Icon from "../components/Icon.vue";
import Photos from "../components/Photos.vue";
import Modal from "../components/Modal.vue";
const route = useRoute(),
  router = useRouter();
const id = String(route.params.id || "");
const entry = ref<Entry>(
  blank(route.path.startsWith("/pantry") ? "pantry" : "recipe"),
);
const loading = ref(true),
  saving = ref(false),
  photosBusy = ref(false),
  error = ref(""),
  ready = ref(false),
  saved = ref(false),
  restored = ref(false),
  conflict = ref(false),
  storageKey = "draft:" + (id || entry.value.kind);
const snapshot = ref("");
const tagInput = ref("");
const pending = ref(
  stored<{ body: string; key: string }>(
    "pending:records" + (id ? "/" + id : ""),
  ),
);
const staleDraft = ref<Entry | null>(null);
const reviewLatest = ref(false);
const leaveTo = ref("");
const allowLeave = ref(false);
const bulk = ref(false),
  bulkText = ref("");
const isPantry = computed(() => entry.value.kind === "pantry");
const dirty = computed(
  () => ready.value && JSON.stringify(entry.value) !== snapshot.value,
);
const categories = computed(() =>
  isPantry.value ? pantryCategories : recipeCategories,
);
onMounted(async () => {
  try {
    if (pending.value) {
      try {
        const result = await api<Entry>("operations/" + pending.value.key);
        saved.value = true;
        forget(storageKey);
        forget("pending:records" + (id ? "/" + id : ""));
        notify("上次保存已成功，已找回记录");
        await router.replace(
          result.kind === "pantry" ? "/pantry" : "/recipes/" + result.id,
        );
        return;
      } catch (e) {
        if (!(e instanceof ApiError && e.status === 404)) throw e;
      }
    }
    if (id) entry.value = await api<Entry>("records/" + id);
    if (entry.value.deletedAt) {
      error.value = "这条记录在回收站中，请先恢复。";
      return;
    }
    snapshot.value = JSON.stringify(entry.value);
    const draft = stored<{ entry: Entry; at: number }>(storageKey);
    if (draft && Date.now() - draft.at < 86400000) {
      if (draft.entry.revision === entry.value.revision) {
        entry.value = draft.entry;
        restored.value = true;
      } else {
        staleDraft.value = draft.entry;
        error.value =
          "草稿对应的记录已更新。可以先查看旧草稿，或基于最新内容继续编辑。";
      }
    }
    if (!entry.value.ingredients.length)
      entry.value.ingredients.push({ name: "", amount: "" });
    if (!entry.value.steps.length) entry.value.steps.push("");
    snapshot.value = restored.value
      ? snapshot.value
      : JSON.stringify(entry.value);
    ready.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
});
watch(
  entry,
  () => {
    if (ready.value && !saved.value)
      persist(storageKey, { entry: entry.value, at: Date.now() });
  },
  { deep: true },
);
function beforeUnload(e: BeforeUnloadEvent) {
  if ((dirty.value || photosBusy.value) && !saved.value) {
    e.preventDefault();
    e.returnValue = "";
  }
}
window.addEventListener("beforeunload", beforeUnload);
onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));
onBeforeRouteLeave((to) => {
  if (!saved.value && !allowLeave.value && (dirty.value || photosBusy.value)) {
    leaveTo.value = to.fullPath;
    return false;
  }
  return true;
});
function addTag() {
  const parts = tagInput.value
    .split(/[,，、\n]/)
    .map((x) => x.trim())
    .filter(Boolean);
  entry.value.tags = [...new Set([...entry.value.tags, ...parts])];
  tagInput.value = "";
}
function move<T>(items: T[], i: number, offset: number) {
  const [v] = items.splice(i, 1);
  items.splice(i + offset, 0, v);
}
function pasteIngredients() {
  const items = bulkText.value
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean)
    .map((line) => {
      const [name, ...rest] = line.split(/\s+/);
      return { name, amount: rest.join(" ") };
    });
  entry.value.ingredients = [
    ...entry.value.ingredients.filter((x) => x.name || x.amount),
    ...items,
  ];
  bulk.value = false;
  bulkText.value = "";
}
async function save() {
  if (saving.value || photosBusy.value) return;
  addTag();
  saving.value = true;
  error.value = "";
  conflict.value = false;
  try {
    const data = pending.value
      ? JSON.parse(pending.value.body)
      : JSON.parse(JSON.stringify(entry.value));
    for (const field of ["minutes", "servings", "quantity"])
      data[field] = Number(data[field] || 0);
    const result = await write(id, data);
    saved.value = true;
    forget(storageKey);
    notify(isPantry.value ? "常备食品已保存" : "这道家的味道，记下了");
    router.push(isPantry.value ? "/pantry" : "/recipes/" + result.id);
  } catch (e) {
    error.value = (e as Error).message;
    conflict.value = e instanceof ApiError && e.status === 409;
    pending.value = stored("pending:records" + (id ? "/" + id : ""));
  } finally {
    saving.value = false;
  }
}
async function compareLatest() {
  reviewLatest.value = false;
  try {
    const current = await api<Entry>("records/" + id);
    entry.value.revision = current.revision;
    entry.value.logs = current.logs;
    conflict.value = false;
    error.value = "已对齐最新版本。请核对其他设备上的内容后，再保存当前草稿。";
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>
<template>
  <div class="page editor-page">
    <RouterLink
      :to="isPantry ? '/pantry' : id ? '/recipes/' + id : '/'"
      class="back-link"
      ><Icon name="arrow" :size="18" />{{
        isPantry ? "常备食品" : "我的菜谱"
      }}</RouterLink
    >
    <div class="page-title">
      <div class="eyebrow">
        {{ isPantry ? "A WELL-STOCKED KITCHEN" : "A RECIPE TO REMEMBER" }}
      </div>
      <h1>
        {{ id ? "编辑" : isPantry ? "添加" : "记一道新"
        }}{{ isPantry ? "食品" : id ? "菜谱" : "菜" }}
      </h1>
      <p>
        {{
          isPantry
            ? "把买好的食品记下来，吃得安心，也不容易忘。"
            : "先写下菜名就能保存，其他的慢慢补齐。"
        }}
      </p>
    </div>
    <div v-if="loading" class="empty">正在准备…</div>
    <form v-else-if="ready" @submit.prevent="save">
      <div v-if="restored" class="notice">已恢复上次未保存的草稿。</div>
      <div v-if="pending" class="notice">
        上次保存结果尚未确认。请点击“核对上次保存”，期间草稿会保持原样。
      </div>
      <div v-if="staleDraft" class="notice">
        存在较早版本的未提交草稿。<button
          type="button"
          class="text-btn"
          @click="reviewLatest = true"
        >
          查看旧草稿
        </button>
      </div>
      <div class="editor-columns" :inert="!!pending">
        <div class="editor-primary">
          <section class="panel">
            <div class="section-heading">
              <span class="section-number">01</span>
              <h2>基本信息</h2>
              <span>必填一项就好</span>
            </div>
            <label class="field"
              >{{ isPantry ? "食品名称" : "菜名" }}
              <span class="required">*</span
              ><input
                v-model="entry.name"
                required
                maxlength="120"
                :placeholder="
                  isPantry ? '例如：番茄鸡蛋挂面' : '例如：妈妈的番茄炒蛋'
                "
                class="name-input"
            /></label>
            <div class="form-grid">
              <label class="field"
                >分类<input
                  v-model="entry.category"
                  list="categories"
                  maxlength="40" /><datalist id="categories">
                  <option
                    v-for="c in categories"
                    :value="c"
                  /></datalist></label
              ><label class="field" v-if="!isPantry"
                >大约用时（分钟）<input
                  type="number"
                  inputmode="numeric"
                  :value="entry.minutes || ''"
                  @input="
                    entry.minutes = Number(
                      ($event.target as HTMLInputElement).value,
                    )
                  "
                  min="0"
                  max="10080"
                  placeholder="可不填"
              /></label>
            </div>
            <label v-if="!isPantry" class="field"
              >适合几人吃<input
                type="number"
                inputmode="numeric"
                :value="entry.servings || ''"
                @input="
                  entry.servings = Number(
                    ($event.target as HTMLInputElement).value,
                  )
                "
                min="0"
                max="100"
                placeholder="可不填" /></label
            ><template v-if="isPantry"
              ><div class="form-grid">
                <label class="field"
                  >剩余数量<input
                    type="number"
                    inputmode="numeric"
                    v-model.number="entry.quantity"
                    required
                    min="0"
                    max="9999" /></label
                ><label class="field"
                  >单位<input
                    v-model="entry.unit"
                    list="units"
                    maxlength="100"
                    placeholder="包 / 袋 / 瓶" /><datalist id="units">
                    <option
                      v-for="u in ['包', '袋', '瓶', '盒', '罐', '份']"
                      :value="u"
                    /></datalist
                ></label>
              </div>
              <label class="field"
                >放在哪里<input
                  v-model="entry.location"
                  maxlength="100"
                  placeholder="例如：厨房上层橱柜"
              /></label>
              <div class="form-grid dates">
                <label class="field"
                  >购买日期<span class="date-input"
                    ><input
                      type="date"
                      v-model="entry.purchaseDate" /></span></label
                ><label class="field"
                  >到期日期<span class="date-input"
                    ><input
                      type="date"
                      v-model="entry.expiryDate"
                      :min="entry.purchaseDate || undefined" /></span
                ></label></div></template
            ><label class="field"
              >{{ isPantry ? "备注" : "这道菜的小故事"
              }}<textarea
                v-model="entry.notes"
                rows="3"
                maxlength="10000"
                :placeholder="
                  isPantry
                    ? '口味、开封提醒、购买链接…'
                    : '是谁的拿手菜？有什么小诀窍？'
                "
              /></label
            ><template v-if="!isPantry"
              ><label class="field"
                >标签
                <div class="tag-input">
                  <input
                    v-model="tagInput"
                    placeholder="如：下饭、快手、孩子爱吃"
                    @keydown.enter.prevent="addTag"
                    maxlength="200"
                  /><button type="button" class="btn secondary" @click="addTag">
                    添加
                  </button>
                </div></label
              >
              <div class="chip-row">
                <button
                  v-for="tag in entry.tags"
                  type="button"
                  class="chip active"
                  @click="entry.tags = entry.tags.filter((t) => t !== tag)"
                >
                  {{ tag }}<Icon name="close" :size="14" />
                </button>
              </div>
              <label class="check-field"
                ><input type="checkbox" v-model="entry.favorite" /><Icon
                  name="heart"
                />收藏为拿手好菜</label
              ></template
            >
          </section>
          <section class="panel" v-if="!isPantry">
            <div class="section-heading">
              <span class="section-number">02</span>
              <h2>准备食材</h2>
              <button class="text-btn" type="button" @click="bulk = true">
                批量粘贴
              </button>
            </div>
            <div class="ingredients-labels">
              <span>食材</span><span>用量</span>
            </div>
            <div
              class="ingredient-edit"
              v-for="(item, i) in entry.ingredients"
              :key="i"
            >
              <input
                v-model="item.name"
                :aria-label="'食材 ' + (i + 1)"
                placeholder="番茄"
                maxlength="100"
              /><input
                v-model="item.amount"
                :aria-label="'用量 ' + (i + 1)"
                placeholder="2 个 / 适量"
                maxlength="100"
              /><button
                type="button"
                class="icon-btn"
                :aria-label="'移除食材 ' + (i + 1)"
                @click="entry.ingredients.splice(i, 1)"
              >
                <Icon name="close" :size="17" />
              </button>
            </div>
            <button
              class="add-row"
              type="button"
              @click="entry.ingredients.push({ name: '', amount: '' })"
              :disabled="entry.ingredients.length >= 100"
            >
              <Icon name="plus" :size="17" />添加食材
            </button>
          </section>
          <section class="panel" v-if="!isPantry">
            <div class="section-heading">
              <span class="section-number">03</span>
              <h2>开始做菜</h2>
              <span>一步一步，记清楚</span>
            </div>
            <div class="step-edit" v-for="(_, i) in entry.steps" :key="i">
              <div class="step-top">
                <span>步骤 {{ String(i + 1).padStart(2, "0") }}</span>
                <div>
                  <button
                    type="button"
                    class="icon-btn"
                    :aria-label="'上移步骤 ' + (i + 1)"
                    :disabled="i === 0"
                    @click="move(entry.steps, i, -1)"
                  >
                    <Icon name="up" :size="17" /></button
                  ><button
                    type="button"
                    class="icon-btn"
                    :aria-label="'下移步骤 ' + (i + 1)"
                    :disabled="i === entry.steps.length - 1"
                    @click="move(entry.steps, i, 1)"
                  >
                    <Icon name="down" :size="17" /></button
                  ><button
                    type="button"
                    class="icon-btn"
                    :aria-label="'移除步骤 ' + (i + 1)"
                    @click="entry.steps.splice(i, 1)"
                  >
                    <Icon name="close" :size="17" />
                  </button>
                </div>
              </div>
              <textarea
                v-model="entry.steps[i]"
                rows="3"
                :aria-label="'步骤 ' + (i + 1)"
                maxlength="4000"
                placeholder="写下做法，也可以加上火候、时间和小窍门…"
              />
            </div>
            <button
              class="add-row"
              type="button"
              @click="entry.steps.push('')"
              :disabled="entry.steps.length >= 100"
            >
              <Icon name="plus" :size="17" />添加步骤
            </button>
          </section>
        </div>
        <aside class="editor-side">
          <section class="panel">
            <div class="section-heading">
              <Icon name="camera" />
              <h2>{{ isPantry ? "食品照片" : "美味留影" }}</h2>
              <span>选填</span>
            </div>
            <Photos v-model="entry.photoIds" @busy="photosBusy = $event" />
          </section>
          <div class="handwritten-note">
            <Icon name="leaf" :size="24" />
            <p>
              {{
                isPantry
                  ? "看得见的储备，让每一份食物都被好好享用。"
                  : "不用像大厨一样精确，适量、少许、凭感觉，也都是家的配方。"
              }}
            </p>
          </div>
        </aside>
      </div>
      <div v-if="error" class="notice error" role="alert">
        {{ error
        }}<button
          v-if="conflict"
          type="button"
          class="btn secondary"
          @click="
            staleDraft = JSON.parse(JSON.stringify(entry));
            reviewLatest = true;
          "
        >
          对照草稿与最新版本
        </button>
      </div>
      <div class="save-bar">
        <span>{{
          photosBusy
            ? "请等待照片上传，失败的照片请重试或移除"
            : saving
              ? "正在保存…"
              : "文字与已上传照片会暂存为草稿"
        }}</span
        ><button
          class="btn primary"
          type="submit"
          :disabled="saving || photosBusy"
        >
          <Icon name="check" />{{
            saving
              ? "保存中…"
              : pending
                ? "核对上次保存"
                : isPantry
                  ? "保存食品"
                  : "保存菜谱"
          }}
        </button>
      </div>
    </form>
    <div v-else-if="error" class="notice error">{{ error }}</div>
    <Modal
      v-if="reviewLatest"
      title="核对两份内容"
      wide
      @close="reviewLatest = false"
      ><p class="muted">
        先在另一标签页打开最新记录进行核对。直接保存旧草稿会覆盖同名字段，请先把需要保留的内容合并到表单。
      </p>
      <a
        v-if="id"
        class="btn secondary"
        :href="'/recipebox/recipes/' + id"
        target="_blank"
        rel="noopener"
        >打开最新记录</a
      >
      <details open>
        <summary>本机保留的草稿</summary>
        <div v-if="staleDraft" class="draft-preview">
          <h3>{{ staleDraft.name }}</h3>
          <p>
            {{ staleDraft.category
            }}<template v-if="staleDraft.minutes">
              · {{ staleDraft.minutes }} 分钟</template
            ><template v-if="staleDraft.servings">
              · {{ staleDraft.servings }} 人份</template
            >
          </p>
          <p>{{ staleDraft.notes || "未填写备注" }}</p>
          <template v-if="staleDraft.kind === 'pantry'"
            ><p>数量：{{ staleDraft.quantity }} {{ staleDraft.unit }}</p>
            <p>位置：{{ staleDraft.location || "未填写" }}</p>
            <p>
              购买日期：{{ staleDraft.purchaseDate || "未填写" }} · 到期：{{
                staleDraft.expiryDate || "未填写"
              }}
            </p></template
          ><template v-else
            ><p>标签：{{ staleDraft.tags.join("、") || "无" }}</p>
            <h4>食材</h4>
            <p v-for="i in staleDraft.ingredients">
              {{ i.name }} {{ i.amount }}
            </p>
            <h4>做法</h4>
            <p v-for="(text, i) in staleDraft.steps">
              {{ i + 1 }}. {{ text }}
            </p></template
          >
          <p>保留照片 {{ staleDraft.photoIds.length }} 张</p>
        </div>
      </details>
      <div class="modal-actions">
        <button class="btn secondary" @click="reviewLatest = false">
          返回编辑</button
        ><button
          class="btn primary"
          @click="
            async () => {
              if (staleDraft) {
                entry = JSON.parse(JSON.stringify(staleDraft));
                await compareLatest();
                staleDraft = null;
              }
            }
          "
        >
          保留此草稿，继续合并
        </button>
      </div></Modal
    ><Modal v-if="bulk" title="批量添加食材" @close="bulk = false"
      ><p class="muted">每行一种食材，名称和用量之间留空格。</p>
      <textarea
        v-model="bulkText"
        rows="8"
        placeholder="番茄 2 个&#10;鸡蛋 3 个&#10;盐 适量"
        aria-label="批量食材"
      ></textarea>
      <div class="modal-actions">
        <button class="btn primary" @click="pasteIngredients">
          添加到食材清单
        </button>
      </div></Modal
    ><Modal v-if="leaveTo" title="暂时离开？" @close="leaveTo = ''"
      ><p>
        文字和已上传的照片已保留为草稿。{{
          photosBusy
            ? "尚未完成上传的照片需要重新选择。"
            : "下次打开可以继续编辑。"
        }}
      </p>
      <div class="modal-actions">
        <button class="btn secondary" @click="leaveTo = ''">继续编辑</button
        ><button
          class="btn primary"
          @click="
            allowLeave = true;
            router.push(leaveTo);
          "
        >
          保留草稿并离开
        </button>
      </div></Modal
    >
  </div>
</template>
