<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute } from 'vue-router';
import { getApiErrorCode } from '@/service/api/errors';
import { useRouterPush } from '@/hooks/common/router';

const route = useRoute();
const { routerPushByKey } = useRouterPush();

const projectId = computed(() => String(route.params.id ?? ''));

/** the error thrown while loading the project, the wizard reports it */
const loadError = ref<unknown>(null);

const notFound = computed(() => getApiErrorCode(loadError.value) === 'not_found');

const resultStatus = computed<'404' | 'error'>(() => (notFound.value ? '404' : 'error'));
</script>

<template>
  <div class="flex-col-stretch gap-16px">
    <NCard :bordered="false" class="card-wrapper" size="small">
    <h2 class="mb-16px text-18px font-500">编辑项目</h2>

    <ProjectWizard v-if="!loadError" :project-id="projectId" @load-error="loadError = $event" />

    <NResult
      v-else
      :status="resultStatus"
      :title="notFound ? '项目不存在' : '项目加载失败'"
      :description="notFound ? '该项目可能已被删除，或链接中的 id 不正确。' : '加载项目时发生错误，请稍后重试。'"
    >
      <template #footer>
        <NSpace justify="center">
          <NButton @click="loadError = null">重试</NButton>
          <NButton type="primary" @click="routerPushByKey('project-list')">返回项目列表</NButton>
        </NSpace>
      </template>
    </NResult>
    </NCard>
  </div>
</template>

<style scoped></style>
