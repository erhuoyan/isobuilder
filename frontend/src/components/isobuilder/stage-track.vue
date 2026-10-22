<script setup lang="ts">
import { computed } from 'vue';
import { buildStages } from '@/constants/isobuilder';

interface Props {
  /** the current stage of the build, empty when it has not started */
  stage: Api.IsoBuilder.BuildStage;
  /** the 1 based index of the current stage, 0 when the build has not started */
  stageIndex: number;
  /** the total number of stages */
  stageTotal: number;
  /** the progress from 0 to 100 */
  progress: number;
  status: Api.IsoBuilder.BuildStatus;
}

const props = defineProps<Props>();

type StageState = 'done' | 'active' | 'error' | 'canceled' | 'pending';

/**
 * The state of every stage, derived from the build status and the current stage index
 *
 * A failed build is highlighted on the stage it broke down in.
 */
const stages = computed(() => {
  const total = props.stageTotal || buildStages.length;

  return buildStages.slice(0, total).map((item, index) => {
    const order = index + 1;
    let state: StageState = 'pending';

    if (props.status === 'succeeded') {
      state = 'done';
    } else if (props.status === 'failed') {
      state = order < props.stageIndex ? 'done' : order === props.stageIndex ? 'error' : 'pending';
    } else if (props.status === 'canceled') {
      state = order < props.stageIndex ? 'done' : order === props.stageIndex ? 'canceled' : 'pending';
    } else if (order < props.stageIndex) {
      state = 'done';
    } else if (order === props.stageIndex) {
      state = 'active';
    }

    return { ...item, order, state };
  });
});

const indicatorStatus = computed(() => (props.status === 'failed' ? 'error' : props.status === 'canceled' ? 'warning' : 'default'));
</script>

<template>
  <div class="stage-track" role="list" aria-label="构建阶段">
    <div v-for="item in stages" :key="item.key" class="stage-track__item" :class="`is-${item.state}`" role="listitem">
      <div class="stage-track__rail">
        <span class="stage-track__dot">
          <SvgIcon v-if="item.state === 'done'" icon="mdi:check-bold" />
          <SvgIcon v-else-if="item.state === 'active'" icon="mdi:loading" class="animate-spin" />
          <SvgIcon v-else-if="item.state === 'error'" icon="mdi:close-thick" />
          <SvgIcon v-else-if="item.state === 'canceled'" icon="mdi:stop" />
          <span v-else class="text-12px">{{ item.order }}</span>
        </span>
      </div>
      <div class="stage-track__label" :title="item.description">{{ item.label }}</div>
    </div>
  </div>

  <div class="mt-4 flex items-center gap-3">
    <NProgress
      class="min-w-0 flex-1"
      type="line"
      :percentage="progress"
      :status="indicatorStatus"
      :height="8"
      :show-indicator="false"
      :border-radius="4"
    />
    <span class="shrink-0 text-13px font-mono text-gray-500">{{ progress }}%</span>
  </div>
</template>

<style lang="scss" scoped>
.stage-track {
  display: flex;
  align-items: flex-start;
  width: 100%;
}

.stage-track__item {
  position: relative;
  flex: 1 1 0;
  min-width: 0;
  text-align: center;
}

.stage-track__rail {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 28px;

  &::before,
  &::after {
    position: absolute;
    top: 50%;
    width: calc(50% - 16px);
    height: 2px;
    background-color: rgb(var(--gray-300-color));
    content: '';
  }

  &::before {
    left: 0;
  }

  &::after {
    right: 0;
  }
}

.stage-track__item:first-child .stage-track__rail::before,
.stage-track__item:last-child .stage-track__rail::after {
  display: none;
}

.stage-track__dot {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 2px solid rgb(var(--gray-300-color));
  border-radius: 50%;
  background-color: rgb(var(--container-bg-color));
  color: rgb(var(--gray-500-color));
  font-size: 14px;
}

.stage-track__label {
  margin-top: 6px;
  overflow: hidden;
  color: rgb(var(--gray-500-color));
  font-size: 12px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.is-done {
  .stage-track__dot {
    border-color: rgb(var(--success-color));
    background-color: rgb(var(--success-color));
    color: #fff;
  }

  .stage-track__rail::before,
  .stage-track__rail::after {
    background-color: rgb(var(--success-color));
  }

  .stage-track__label {
    color: rgb(var(--base-text-color));
  }
}

.is-active {
  .stage-track__dot {
    border-color: rgb(var(--primary-color));
    background-color: rgb(var(--primary-color));
    color: #fff;
    box-shadow: 0 0 0 4px rgb(var(--primary-color) / 15%);
  }

  .stage-track__rail::before {
    background-color: rgb(var(--primary-color));
  }

  .stage-track__label {
    color: rgb(var(--primary-color));
    font-weight: 500;
  }
}

.is-error {
  .stage-track__dot {
    border-color: rgb(var(--error-color));
    background-color: rgb(var(--error-color));
    color: #fff;
  }

  .stage-track__rail::before {
    background-color: rgb(var(--error-color));
  }

  .stage-track__label {
    color: rgb(var(--error-color));
    font-weight: 500;
  }
}

.is-canceled {
  .stage-track__dot {
    border-color: rgb(var(--warning-color));
    background-color: rgb(var(--warning-color));
    color: #fff;
  }

  .stage-track__label {
    color: rgb(var(--warning-color));
    font-weight: 500;
  }
}
</style>
