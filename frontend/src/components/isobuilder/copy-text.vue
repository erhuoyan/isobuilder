<script setup lang="ts">
import { useClipboard } from '@vueuse/core';

interface Props {
  /** the value to display and copy */
  text?: string | null;
  /** the label in front of the value */
  label?: string;
  /** whether the value may wrap */
  wrap?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  text: '',
  label: '',
  wrap: false
});

const { copy, isSupported } = useClipboard({ legacy: true });

async function handleCopy() {
  if (!props.text) return;

  if (!isSupported.value) {
    window.$message?.warning('当前浏览器不支持剪贴板，请手动复制。');
    return;
  }

  await copy(props.text);
  window.$message?.success('已复制到剪贴板');
}
</script>

<template>
  <div class="min-w-0 flex items-center gap-2">
    <span v-if="label" class="shrink-0 text-14px text-gray-500">{{ label }}</span>
    <code
      class="min-w-0 flex-1 border border-gray-200 rounded-4px bg-gray-50 px-2 py-0.5 text-13px font-mono dark:border-gray-700 dark:bg-gray-900"
      :class="wrap ? 'break-all whitespace-pre-wrap' : 'overflow-x-auto whitespace-nowrap'"
    >
      {{ text || '—' }}
    </code>
    <NButton size="tiny" quaternary :disabled="!text" @click="handleCopy">
      <template #icon>
        <SvgIcon icon="mdi:content-copy" />
      </template>
      复制
    </NButton>
  </div>
</template>

<style scoped></style>
