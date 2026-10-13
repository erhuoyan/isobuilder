<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { fetchGetIsos, fetchGetKickstarts, fetchGetProject, fetchUpdateProject } from '@/service/api';
import HostsEditor from '@/components/isobuilder/hosts-editor.vue';
import { useRouterPush } from '@/hooks/common/router';

const route = useRoute();
const { routerPushByKey } = useRouterPush();

const projectId = String(route.params.id ?? '');

const loading = ref(true);
const saving = ref(false);
const notFound = ref(false);
const project = ref<Api.IsoBuilder.Project | null>(null);

const isos = ref<Api.IsoBuilder.Iso[]>([]);
const kickstarts = ref<Api.IsoBuilder.Kickstart[]>([]);

const form = ref({ name: '', description: '', iso_label: '', output_name: '' });
const hostsRaw = ref('');
const hostsFormat = ref<Api.IsoBuilder.HostsFormatInput>('auto');
const hostsResult = ref<Api.IsoBuilder.ValidateHostsResult | null>(null);

/** 基础镜像与应答模板随项目绑定存在，编辑页只展示；如需更换应新建项目 */
const isoLabel = computed(() => {
  const iso = isos.value.find(item => item.id === project.value?.iso_id);
  return iso ? `${iso.name} · ${iso.filename}` : project.value?.iso_id || '—';
});

const kickstartLabel = computed(() => {
  const kickstart = kickstarts.value.find(item => item.id === project.value?.kickstart_id);
  return kickstart ? kickstart.name : project.value?.kickstart_id || '—';
});

const hostHint = computed(() => {
  if (!hostsRaw.value.trim()) return '请填写主机清单。';

  if (!hostsResult.value) return '主机清单尚未校验通过，请等待自动校验完成后重试。';

  if (!hostsResult.value.ok) return `主机清单存在 ${hostsResult.value.errors.length} 处问题，请修正后再保存。`;

  return '';
});

const canSave = computed(() => Boolean(project.value) && Boolean(form.value.name.trim()) && !hostHint.value);

const saveHint = computed(() => (form.value.name.trim() ? hostHint.value : '请填写项目名称。'));

function onValidateHosts(result: Api.IsoBuilder.ValidateHostsResult | null) {
  hostsResult.value = result;
}

async function loadProject() {
  loading.value = true;
  const { data, error } = await fetchGetProject(projectId);
  loading.value = false;

  if (error) {
    notFound.value = true;
    return;
  }

  project.value = data.project;
  form.value = {
    name: data.project.name,
    description: data.project.description,
    iso_label: data.project.iso_label,
    output_name: data.project.output_name
  };
  hostsRaw.value = data.project.hosts_raw;
  hostsFormat.value = data.project.hosts_format;
}

async function handleSave() {
  if (!canSave.value) return;

  saving.value = true;

  const { data, error } = await fetchUpdateProject(projectId, {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    iso_label: form.value.iso_label.trim(),
    output_name: form.value.output_name.trim(),
    hosts_raw: hostsRaw.value,
    hosts_format: hostsFormat.value
  });

  saving.value = false;

  if (error) return;

  window.$message?.success('项目已保存');
  routerPushByKey('project-detail', { params: { id: data.project.id } });
}

function handleCancel() {
  routerPushByKey('project-detail', { params: { id: projectId } });
}

onMounted(async () => {
  await Promise.all([
    fetchGetIsos().then(({ data, error }) => {
      if (!error) isos.value = data.isos;
    }),
    fetchGetKickstarts().then(({ data, error }) => {
      if (!error) kickstarts.value = data.kickstarts;
    }),
    loadProject()
  ]);
});
</script>

<template>
  <NSpin :show="loading">
    <NResult
      v-if="notFound"
      class="card-wrapper"
      status="404"
      title="项目不存在"
      description="该项目可能已被删除。"
    >
      <template #footer>
        <NButton type="primary" @click="routerPushByKey('project-list')">返回项目列表</NButton>
      </template>
    </NResult>

    <NSpace v-else-if="project" vertical :size="16">
      <NCard :bordered="false" class="card-wrapper" size="small" title="编辑项目">
        <NAlert type="info" :bordered="false" class="mb-16px">
          基础镜像与应答模板随项目绑定，不可修改；如需更换请新建项目。
        </NAlert>

        <NDescriptions :column="2" size="small" bordered label-placement="left" class="mb-16px">
          <NDescriptionsItem label="目标架构">{{ project.arch }}</NDescriptionsItem>
          <NDescriptionsItem label="基础镜像">{{ isoLabel }}</NDescriptionsItem>
          <NDescriptionsItem label="应答模板" :span="2">{{ kickstartLabel }}</NDescriptionsItem>
        </NDescriptions>

        <NForm size="small" label-placement="top" :show-feedback="false">
          <NGrid :cols="2" :x-gap="16">
            <NFormItemGi label="项目名称">
              <NInput v-model:value="form.name" maxlength="100" show-count placeholder="例如：京西机房 perf 批次" />
            </NFormItemGi>
            <NFormItemGi label="描述">
              <NInput v-model:value="form.description" placeholder="选填，说明这批主机的用途、机房或批次" />
            </NFormItemGi>
          </NGrid>
        </NForm>

        <NCollapse class="mt-4px">
          <NCollapseItem title="高级项（ISO 卷标与产物文件名）" name="advanced">
            <NForm size="small" label-placement="top" :show-feedback="false">
              <NGrid :cols="2" :x-gap="16">
                <NFormItemGi label="ISO 卷标">
                  <NInput v-model:value="form.iso_label" placeholder="留空 → 构建时从原版 ISO 自动探测" />
                </NFormItemGi>
                <NFormItemGi label="产物文件名">
                  <NInput v-model:value="form.output_name" placeholder="留空 → 构建时自动生成" />
                </NFormItemGi>
              </NGrid>
            </NForm>
          </NCollapseItem>
        </NCollapse>
      </NCard>

      <NCard :bordered="false" class="card-wrapper" size="small" title="主机载荷">
        <HostsEditor v-model:value="hostsRaw" v-model:format="hostsFormat" @validate="onValidateHosts" />
      </NCard>

      <div class="flex items-center justify-between">
        <span class="text-13px text-gray-400">{{ canSave ? '' : saveHint }}</span>
        <NSpace :size="8">
          <NButton @click="handleCancel">取消</NButton>
          <NButton type="primary" :loading="saving" :disabled="!canSave" @click="handleSave">保存修改</NButton>
        </NSpace>
      </div>
    </NSpace>
  </NSpin>
</template>

<style scoped></style>
