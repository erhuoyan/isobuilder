<script setup lang="ts">
import { computed } from 'vue';
import { isoStatusRecord } from '@/constants/isobuilder';

interface Props {
  status: Api.IsoBuilder.IsoStatus;
  /** whether the tag shows a spinner while the iso is being processed */
  animated?: boolean;
}

const props = withDefaults(defineProps<Props>(), { animated: true });

const record = computed(() => isoStatusRecord[props.status] || { label: props.status, type: 'default' as const });

const processing = computed(() => props.status === 'downloading' || props.status === 'hashing');
</script>

<template>
  <NTag :type="record.type" size="small" :bordered="false" round>
    <template v-if="animated && processing" #icon>
      <SvgIcon icon="mdi:loading" class="animate-spin" />
    </template>
    {{ record.label }}
  </NTag>
</template>

<style scoped></style>
