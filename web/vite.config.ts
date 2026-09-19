import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
export default defineConfig({base:'/recipebox/',plugins:[vue()],server:{proxy:{'/recipebox/api':{target:'http://127.0.0.1:18083',rewrite:path=>path.replace('/recipebox','')}}}});
