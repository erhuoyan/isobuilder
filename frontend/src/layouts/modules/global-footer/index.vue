<script setup lang="ts">
import { useConfigStore } from '@/store/modules/config';

defineOptions({
  name: 'GlobalFooter'
});

const configStore = useConfigStore();
const appVersion = APP_VERSION;
</script>

<template>
  <DarkModeContainer class="h-full flex-center gap-3">
    <span>ISO Builder · 自动装机镜像构建平台</span>
    <span class="text-12px op-40">
      前端 v{{ appVersion }}
      · 后端 v{{ configStore.config?.version ?? '…' }}
      <template v-if="configStore.config?.git_commit">({{ configStore.config.git_commit }})</template>
    </span>
  </DarkModeContainer>
</template>

<style scoped></style>
