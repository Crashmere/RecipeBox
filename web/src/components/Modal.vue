<script setup lang="ts">
import {onMounted,onBeforeUnmount,ref} from 'vue';import Icon from './Icon.vue';
defineProps<{title:string;wide?:boolean}>();const emit=defineEmits<{close:[]}>();const dialog=ref<HTMLDialogElement>();let previous:HTMLElement|null=null;let overflow='';
onMounted(()=>{previous=document.activeElement as HTMLElement;overflow=document.body.style.overflow;document.body.style.overflow='hidden';dialog.value?.showModal()});onBeforeUnmount(()=>{document.body.style.overflow=overflow;previous?.focus()});
</script>
<template><dialog ref="dialog" class="modal" :class="{wide}" @cancel.prevent="emit('close')" @click="($event.target===dialog)&&emit('close')"><div class="modal-inner"><header class="modal-head"><h2>{{title}}</h2><button class="icon-btn" aria-label="关闭" @click="emit('close')"><Icon name="close"/></button></header><slot/></div></dialog></template>
