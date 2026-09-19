<script setup lang="ts">
import { ref, onBeforeUnmount, watch } from "vue";
import { api, media, uid } from "../api";
import Modal from "./Modal.vue";
import Icon from "./Icon.vue";
const props = defineProps<{ modelValue: string[] }>();
const emit = defineEmits<{
  "update:modelValue": [string[]];
  busy: [boolean];
}>();
type Pending = {
  key: string;
  file: File;
  preview: string;
  error: string;
  progress: number;
};
const pending = ref<Pending[]>([]);
const active = ref(false);
const preview = ref("");
const input = ref<HTMLInputElement>();
const camera = ref<HTMLInputElement>();
watch(
  [pending, active],
  () => emit("busy", pending.value.length > 0 || active.value),
  { deep: true },
);
async function send(p: Pending) {
  p.error = "";
  active.value = true;
  try {
    const m = await new Promise<{ id: string }>((resolve, reject) => {
      const x = new XMLHttpRequest();
      x.open("POST", import.meta.env.BASE_URL + "api/uploads");
      x.setRequestHeader("Idempotency-Key", p.key);
      x.timeout = 120000;
      x.upload.onprogress = (e) => {
        if (e.lengthComputable)
          p.progress = Math.round((e.loaded / e.total) * 95);
      };
      x.onload = () => {
        let data;
        try {
          data = JSON.parse(x.responseText);
        } catch {}
        x.status >= 200 && x.status < 300
          ? resolve(data)
          : reject(new Error(data?.message || "上传失败，请重试"));
      };
      x.onerror = () => reject(new Error("连接中断，点击重试"));
      x.ontimeout = () => reject(new Error("上传超时，点击重试"));
      x.send(p.file);
    });
    emit("update:modelValue", [...props.modelValue, m.id]);
    pending.value = pending.value.filter((x) => x.key !== p.key);
    URL.revokeObjectURL(p.preview);
  } catch (e) {
    p.error = (e as Error).message;
  } finally {
    active.value = false;
  }
}
async function select(event: Event) {
  const el = event.target as HTMLInputElement;
  const files = Array.from(el.files || []);
  el.value = "";
  for (const f of files) {
    if (props.modelValue.length + pending.value.length >= 10) break;
    const p: Pending = {
      file: f,
      key: uid(),
      preview: URL.createObjectURL(f),
      error: "",
      progress: 0,
    };
    pending.value.push(p);
    const item = pending.value[pending.value.length - 1];
    if (f.size > 25 * 1024 * 1024) {
      item.error = "超过 25 MB，请移除后换一张";
    } else await send(item);
  }
}
function removePending(p: Pending) {
  pending.value = pending.value.filter((x) => x.key !== p.key);
  URL.revokeObjectURL(p.preview);
}
function move(i: number) {
  const a = [...props.modelValue];
  const [id] = a.splice(i, 1);
  a.unshift(id);
  emit("update:modelValue", a);
}
onBeforeUnmount(() =>
  pending.value.forEach((p) => URL.revokeObjectURL(p.preview)),
);
</script>
<template>
  <div class="photo-editor">
    <div class="photo-grid" v-if="modelValue.length || pending.length">
      <div v-for="(id, i) in modelValue" :key="id" class="photo-tile">
        <button
          type="button"
          class="photo-preview"
          aria-label="预览照片"
          @click="preview = media(id)"
        >
          <img :src="media(id, true)" alt="菜谱照片" /></button
        ><span class="cover-tag" v-if="i === 0">封面</span>
        <div class="photo-actions">
          <button type="button" @click="move(i)" :disabled="i === 0">
            设为封面</button
          ><button
            type="button"
            :aria-label="'移除照片 ' + (i + 1)"
            @click="
              emit(
                'update:modelValue',
                modelValue.filter((x) => x !== id),
              )
            "
          >
            <Icon name="close" :size="16" />
          </button>
        </div>
      </div>
      <div v-for="p in pending" :key="p.key" class="photo-tile pending">
        <img :src="p.preview" alt="待上传照片" />
        <div class="upload-state">
          <span>{{ p.error || "上传中 " + p.progress + "%" }}</span
          ><button
            type="button"
            v-if="p.error"
            @click="send(p)"
            :disabled="active"
          >
            重试</button
          ><button type="button" v-if="p.error" @click="removePending(p)">
            移除
          </button>
        </div>
      </div>
    </div>
    <div class="upload-zone" v-if="modelValue.length + pending.length < 10">
      <Icon name="image" :size="30" />
      <p>
        {{ modelValue.length ? "再留下一张美味瞬间" : "给这道菜留一张照片" }}
      </p>
      <div class="button-row">
        <button
          type="button"
          class="btn secondary"
          :disabled="active"
          @click="input?.click()"
        >
          <Icon name="image" />从相册选择</button
        ><button
          type="button"
          class="btn secondary"
          :disabled="active"
          @click="camera?.click()"
        >
          <Icon name="camera" />拍照
        </button>
      </div>
      <small>最多 10 张 · JPG / PNG / WebP / HEIC · 每张 25 MB</small>
    </div>
    <input
      ref="input"
      type="file"
      accept="image/jpeg,image/png,image/webp,image/heic,image/heif,.heic,.heif"
      multiple
      hidden
      @change="select"
    /><input
      ref="camera"
      type="file"
      accept="image/*"
      capture="environment"
      hidden
      @change="select"
    /><Modal v-if="preview" title="照片预览" wide @close="preview = ''"
      ><img class="full-photo" :src="preview" alt="照片预览"
    /></Modal>
  </div>
</template>
