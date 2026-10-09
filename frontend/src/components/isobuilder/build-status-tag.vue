<script setup lang="ts">
import { computed } from 'vue';
import { buildStatusRecord } from '@/constants/isobuilder';

interface Props {
  status: Api.IsoBuilder.BuildStatus;
  /** whether the tag shows a spinner while the build is running */
  animated?: boolean;
}

const props = withDefaults(defineProps<Props>(), { animated: true });

const record = computed(() => buildStatusRecord[props.status] || { label: props.status, type: 'default' as const });
</script>

<template>
  <NTag :type="record.type" size="small" :bordered="false" round>
    <template v-if="animated && status === 'running'" #icon>
      <SvgIcon icon="mdi:loading" class="animate-spin" />
    </template>
    {{ record.label }}
  </NTag>
</template>

<style scoped></style>
