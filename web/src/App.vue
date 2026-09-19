<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute } from "vue-router";
import Icon from "./components/Icon.vue";
import { message, base } from "./api";
const route = useRoute();
const menu = ref(false);
watch(
  () => route.fullPath,
  () => (menu.value = false),
);
</script>
<template>
  <a class="skip" href="#main">跳到内容</a>
  <header class="site-header">
    <div class="header-inner">
      <RouterLink to="/" class="brand"
        ><img :src="base + 'recipebox.svg'" alt="" /><span
          >RecipeBox<small>家的味道，慢慢收藏</small></span
        ></RouterLink
      >
      <nav class="desktop-nav" aria-label="主导航">
        <RouterLink
          to="/"
          :class="{
            selected:
              !route.path.startsWith('/pantry') && route.path != '/trash',
          }"
          ><Icon name="book" />我的菜谱</RouterLink
        ><RouterLink
          to="/pantry"
          :class="{ selected: route.path.startsWith('/pantry') }"
          ><Icon name="box" />常备食品</RouterLink
        >
      </nav>
      <div class="header-end">
        <span class="header-note">好好做饭 · 好好生活</span>
        <div
          class="menu-wrap"
          @focusout="
            (e) => {
              if (
                !(e.currentTarget as HTMLElement).contains(
                  e.relatedTarget as Node,
                )
              )
                menu = false;
            }
          "
          @keydown.esc="menu = false"
        >
          <button
            class="icon-btn"
            aria-label="更多功能"
            :aria-expanded="menu"
            @click="menu = !menu"
          >
            <Icon name="menu" />
          </button>
          <div class="dropdown" v-if="menu">
            <RouterLink to="/trash"><Icon name="trash" />回收站</RouterLink
            ><a :href="base + 'api/export'" @click="menu = false"
              ><Icon name="download" />导出全部记录和照片</a
            >
          </div>
        </div>
      </div>
    </div>
  </header>
  <main id="main"><RouterView :key="route.path" /></main>
  <nav class="mobile-nav" aria-label="手机导航">
    <RouterLink
      to="/"
      :class="{
        selected: !route.path.startsWith('/pantry') && route.path != '/trash',
      }"
      ><Icon name="book" /><span>我的菜谱</span></RouterLink
    ><RouterLink
      to="/pantry"
      :class="{ selected: route.path.startsWith('/pantry') }"
      ><Icon name="box" /><span>常备食品</span></RouterLink
    >
  </nav>
  <Transition name="toast"
    ><div v-if="message" class="toast" role="status">
      <Icon name="check" />{{ message }}
    </div></Transition
  >
</template>
