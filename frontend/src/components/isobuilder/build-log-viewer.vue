<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { streamBuildLog } from '@/service/api';
import { formatBytes } from '@/utils/format';

interface Props {
  buildId: string;
  /** the status of the build, a terminal status means the stream ends after replaying the log */
  status: Api.IsoBuilder.BuildStatus;
  /** the number of bytes written to the log so far */
  logBytes: number;
}

const props = defineProps<Props>();

const emit = defineEmits<{ /** the log stream reached the end of the build */ end: [status: Api.IsoBuilder.BuildStatus] }>();

/** the polling interval of the line buffer, the log may produce thousands of lines per second */
const FLUSH_INTERVAL = 120;
/** the maximum number of lines kept in the view */
const MAX_LINES = 5000;
/** how many times a dropped stream is retried before the user has to reconnect by hand */
const MAX_RETRY = 6;

type StreamState = 'connecting' | 'streaming' | 'ended' | 'error';

const lines = ref<string[]>([]);
const streamState = ref<StreamState>('connecting');
const errorMsg = ref('');
/** the byte offset to resume the stream from after a reconnect */
const nextOffset = ref(0);
const autoScroll = ref(true);
const logBox = ref<HTMLElement | null>(null);

let controller: AbortController | null = null;
let flushTimer: number | null = null;
let retryTimer: number | null = null;
let retryCount = 0;
let stopped = false;
let pending: string[] = [];

const stateRecord: Record<StreamState, { label: string; class: string; icon: string }> = {
  connecting: { label: '连接中', class: 'is-connecting', icon: 'mdi:loading' },
  streaming: { label: '实时输出', class: 'is-streaming', icon: 'mdi:circle-medium' },
  ended: { label: '已结束', class: 'is-ended', icon: 'mdi:check-circle-outline' },
  error: { label: '已断开', class: 'is-error', icon: 'mdi:alert-circle-outline' }
};

const state = computed(() => stateRecord[streamState.value]);

const canRetry = computed(
  () => streamState.value === 'error' && (retryCount >= MAX_RETRY || Boolean(errorMsg.value))
);

function flush() {
  if (!pending.length) return;

  const next = pending;
  pending = [];

  const merged = lines.value.concat(next);

  if (merged.length > MAX_LINES) {
    lines.value = merged.slice(merged.length - MAX_LINES);
  } else {
    lines.value = merged;
  }

  if (autoScroll.value) {
    nextTick(() => {
      const box = logBox.value;

      if (box) box.scrollTop = box.scrollHeight;
    });
  }
}

function handleScroll() {
  const box = logBox.value;

  if (!box) return;

  autoScroll.value = box.scrollHeight - box.scrollTop - box.clientHeight < 40;
}

function scrollToLatest() {
  const box = logBox.value;

  autoScroll.value = true;

  if (box) box.scrollTop = box.scrollHeight;
}

/** connect to the log stream, resuming from `nextOffset` */
async function connect() {
  if (stopped) return;

  controller = new AbortController();
  streamState.value = 'connecting';
  errorMsg.value = '';

  await streamBuildLog(props.buildId, nextOffset.value, controller.signal, {
    onLine(line) {
      // the offset carried by a line is the next resume offset, so a line which does not advance it
      // has already been read
      if (line.offset <= nextOffset.value) return;

      nextOffset.value = line.offset;
      pending.push(line.line.replace(/\r?\n$/, ''));

      if (streamState.value !== 'streaming') streamState.value = 'streaming';
    },
    onEnd(end) {
      flush();
      streamState.value = 'ended';

      if (end.next_offset > nextOffset.value) {
        nextOffset.value = end.next_offset;
      }

      emit('end', end.status);
    },
    onError(message) {
      if (stopped) return;

      flush();
      errorMsg.value = message;
      streamState.value = 'error';

      if (retryCount < MAX_RETRY) {
        retryCount += 1;
        retryTimer = window.setTimeout(() => {
          streamState.value = 'connecting';
          connect();
        }, 1000);
      }
    }
  });
}

/** reconnect by hand, after the automatic retries are exhausted */
function reconnect() {
  retryCount = 0;
  clearTimeout(retryTimer as number);
  connect();
}

function clearView() {
  lines.value = [];
  pending = [];
}

onMounted(() => {
  flushTimer = window.setInterval(flush, FLUSH_INTERVAL);

  connect();
});

onBeforeUnmount(() => {
  stopped = true;

  controller?.abort();
  clearInterval(flushTimer as number);
  clearTimeout(retryTimer as number);
});
</script>

<template>
  <NCard :bordered="false" class="card-wrapper" size="small">
    <template #header>
      <div class="flex flex-wrap items-center gap-8px">
        <span class="text-15px font-500">构建日志</span>
        <span class="log-state" :class="state.class">
          <SvgIcon :icon="state.icon" :class="{ 'animate-spin': streamState === 'connecting' }" />
          {{ state.label }}
        </span>
        <span class="text-12px text-gray-500">
          <span class="font-mono">{{ lines.length }}</span> 行 ·
          <span class="font-mono">{{ formatBytes(logBytes) }}</span>
        </span>
      </div>
    </template>

    <template #header-extra>
      <NSpace :size="8" :wrap="true" justify="end">
        <NButton v-if="canRetry" size="small" type="primary" @click="reconnect">
          <template #icon>
            <SvgIcon icon="mdi:reload" />
          </template>
          重新连接
        </NButton>
        <NButton size="small" quaternary :disabled="!lines.length" @click="clearView">清空视图</NButton>
      </NSpace>
    </template>

    <NAlert v-if="errorMsg" class="mb-12px" type="warning" :bordered="false">
      {{ errorMsg }}
      <span v-if="streamState === 'error' && retryCount < MAX_RETRY" class="ml-4px">（正在自动重连…）</span>
    </NAlert>

    <div class="relative min-w-0">
      <div
        ref="logBox"
        class="log-box h-[50vh] min-h-240px w-full overflow-auto rounded-6px bg-gray-900 px-12px py-8px"
        role="log"
        aria-live="polite"
        aria-atomic="false"
        @scroll.passive="handleScroll"
      >
        <div v-if="!lines.length" class="text-13px text-gray-500">
          {{ status === 'queued' ? '已排队，等待 worker 开始输出日志…' : '暂无日志输出。' }}
        </div>
        <div v-for="(line, index) in lines" :key="index" class="log-line font-mono text-12px">{{ line }}</div>
      </div>

      <NButton
        v-if="!autoScroll"
        class="absolute bottom-12px right-12px"
        size="small"
        type="primary"
        ghost
        @click="scrollToLatest"
      >
        <template #icon>
          <SvgIcon icon="mdi:arrow-down" />
        </template>
        回到最新
      </NButton>
    </div>
  </NCard>
</template>

<style lang="scss" scoped>
.log-state {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 0 8px;
  border-radius: 10px;
  font-size: 12px;
  line-height: 20px;
}

.is-connecting {
  background-color: rgb(var(--info-color) / 12%);
  color: rgb(var(--info-color));
}

.is-streaming {
  background-color: rgb(var(--primary-color) / 12%);
  color: rgb(var(--primary-color));
}

.is-ended {
  background-color: rgb(var(--success-color) / 12%);
  color: rgb(var(--success-color));
}

.is-error {
  background-color: rgb(var(--error-color) / 12%);
  color: rgb(var(--error-color));
}

.log-box {
  scrollbar-width: thin;
}

.log-line {
  min-height: 18px;
  color: #d8e0e8;
  line-height: 18px;
  white-space: pre;
}
</style>
