import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import Library from "./pages/Library.vue";
import Editor from "./pages/Editor.vue";
import Detail from "./pages/Detail.vue";
import Pantry from "./pages/Pantry.vue";
import Trash from "./pages/Trash.vue";
import "./style.css";
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: "/", component: Library },
    { path: "/new", component: Editor },
    { path: "/recipes/:id", component: Detail },
    { path: "/edit/:id", component: Editor },
    { path: "/pantry", component: Pantry },
    { path: "/pantry/new", component: Editor },
    { path: "/trash", component: Trash },
    { path: "/:pathMatch(.*)*", redirect: "/" },
  ],
  scrollBehavior(to, from, saved) {
    return saved || { top: 0 };
  },
});
router.afterEach(() => {
  document.title = "RecipeBox · 家的味道";
});
createApp(App).use(router).mount("#app");
