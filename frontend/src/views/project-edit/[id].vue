<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { fetchGetIsos, fetchGetKickstarts, fetchGetProject, fetchGetScripts, fetchUpdateProject } from '@/service/api';
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

const form = ref({ name: '', description: '', iso_id: '', kickstart_id: '', iso_label: '', output_name: '', payload_mode: 'off' as Api.IsoBuilder.PayloadMode });
const hostsRaw = ref('');
const hostsFormat = ref<Api.IsoBuilder.HostsFormatInput>('auto');
const hostsResult = ref<Api.IsoBuilder.ValidateHostsResult | null>(null);
const postScript = ref('');
/** 脚本库与当前项目选中的脚本 */
const scripts = ref<Api.IsoBuilder.Script[]>([]);
const selectedScriptId = ref('');
const scriptOptions = computed(() => scripts.value.map(item => ({ label: item.name, value: item.id })));

/**
 * 只列与项目架构一致、且已就绪的镜像；当前已绑定的那个即使状态变了也保留，
 * 否则用户会以为绑定丢了。
 */
const isoOptions = computed(() =>
  isos.value
    .filter(iso => iso.arch === project.value?.arch && (iso.status === 'ready' || iso.id === form.value.iso_id))
    .map(iso => ({
      label: `${iso.name} · ${iso.filename}${iso.status === 'ready' ? '' : `（${iso.status}）`}`,
      value: iso.id,
      disabled: iso.status !== 'ready'
    }))
);

const kickstartOptions = computed(() =>
  kickstarts.value
    .filter(ks => ks.arch === 'any' || ks.arch === project.value?.arch)
    .map(ks => ({ label: `${ks.name} · ${ks.arch}`, value: ks.id }))
);

const hostHint = computed(() => {
  if (!hostsRaw.value.trim()) return '请填写主机清单。';

  if (!hostsResult.value) return '主机清单尚未校验通过，请等待自动校验完成后重试。';

  if (!hostsResult.value.ok) return `主机清单存在 ${hostsResult.value.errors.length} 处问题，请修正后再保存。`;

  return '';
});

/**
 * 草稿可保存：只要有名称就能存，镜像/模板/清单可以之后再补。
 * 完整性由构建时把关（后端 project_incomplete 会列出缺什么）。
 */
const canSave = computed(() => Boolean(project.value) && Boolean(form.value.name.trim()));

/** 保存时的提示：能存但还不完整，告诉用户构建前还缺什么 */
const saveHint = computed(() => {
  if (!form.value.name.trim()) return '请填写项目名称。';

  const missing: string[] = [];
  if (!form.value.iso_id) missing.push('基础镜像');
  if (!form.value.kickstart_id) missing.push('应答模板');
  if (!hostsRaw.value.trim()) missing.push('主机清单');
  else if (hostHint.value) missing.push('主机清单校验');

  return missing.length ? `可以先保存草稿；构建前还需补齐：${missing.join('、')}` : '';
});

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

  // The template gates the whole page on `project`, and the label computeds
  // read it too — forgetting this assignment left the view blank.
  project.value = data.project;

  form.value = {
    name: data.project.name,
    description: data.project.description,
    iso_id: data.project.iso_id,
    kickstart_id: data.project.kickstart_id,
    iso_label: data.project.iso_label,
    output_name: data.project.output_name,
    payload_mode: data.project.payload_mode || 'off'
  };
  // hosts_raw 在列表里被后端省略，GET 详情时也可能因 omitempty 缺失
  hostsRaw.value = data.project.hosts_raw ?? '';
  hostsFormat.value = data.project.hosts_format ?? 'auto';
  postScript.value = data.project.post_script;
  selectedScriptId.value = data.project.script_id;
}

async function handleSave() {
  if (!canSave.value) return;

  saving.value = true;

  const { data, error } = await fetchUpdateProject(projectId, {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    iso_id: form.value.iso_id,
    kickstart_id: form.value.kickstart_id,
    iso_label: form.value.iso_label.trim(),
    output_name: form.value.output_name.trim(),
    post_script: postScript.value,
    script_id: selectedScriptId.value,
    payload_mode: form.value.payload_mode,
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
    fetchGetScripts().then(({ data, error }) => {
      if (!error) scripts.value = data.scripts;
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
        <!-- 与模板 basicForm 一致：label 在左、宽度固定，form-item 自带纵向间距 -->
        <n-form
          ref="formRef"
          :model="form"
          :label-width="120"
          label-placement="left"
          size="medium"
          class="py-4"
        >
          <n-form-item label="项目名称" path="name">
            <n-input v-model:value="form.name" maxlength="100" show-count placeholder="例如：京西机房 perf 批次" />
          </n-form-item>
          <n-form-item label="描述" path="description">
            <n-input v-model:value="form.description" placeholder="选填，说明这批主机的用途、机房或批次" />
          </n-form-item>
          <n-form-item :label="`基础镜像`" path="iso_id">
            <n-select
              v-model:value="form.iso_id"
              :options="isoOptions"
              clearable
              placeholder="选择一个状态为「可用」的镜像"
            />
          </n-form-item>
          <n-form-item label="应答模板" path="kickstart_id">
            <n-select
              v-model:value="form.kickstart_id"
              :options="kickstartOptions"
              clearable
              placeholder="选择 kickstart 模板"
            />
          </n-form-item>
          <n-form-item label="当前架构">
            <n-tag :bordered="false" size="small">{{ project.arch }}</n-tag>
          </n-form-item>
        </n-form>

        <NCollapse class="mt-1">
          <NCollapseItem title="高级项（ISO 卷标与产物文件名）" name="advanced">
            <n-form :model="form" :label-width="120" label-placement="left" size="medium" class="py-2">
              <n-form-item label="ISO 卷标">
                <n-input v-model:value="form.iso_label" placeholder="留空则构建时从原版 ISO 自动探测" />
              </n-form-item>
              <n-form-item label="产物文件名">
                <n-input v-model:value="form.output_name" placeholder="留空则构建时自动生成" />
              </n-form-item>
            </n-form>
          </NCollapseItem>
          <NCollapseItem title="自定义脚本" name="post-script">
            <n-form :model="form" :label-width="120" label-placement="left" size="medium" class="py-2">
              <n-form-item label="脚本库">
                <NSelect
                  v-model:value="selectedScriptId"
                  :options="scriptOptions"
                  clearable
                  placeholder="从脚本库选一个；也可只用下方追加脚本"
                />
              </n-form-item>
              <n-form-item label="U 盘注入">
                <NSelect
                  v-model:value="form.payload_mode"
                  :options="[
                    { label: '不注入（默认）', value: 'off' },
                    { label: '按卷标查找 U 盘分区', value: 'label' },
                    { label: '卷标优先，未命中则扫描', value: 'auto' }
                  ]"
                />
              </n-form-item>
              <n-form-item label="追加脚本">
                <NInput
                  v-model:value="postScript"
                  type="textarea"
                  :rows="8"
                  placeholder="留空则不追加。库脚本先执行，这里的内容排在其后。"
                  class="font-mono"
                />
              </n-form-item>
            </n-form>
          </NCollapseItem>
        </NCollapse>
      </NCard>

      <NCard :bordered="false" class="card-wrapper" size="small" title="主机载荷">
        <HostsEditor v-model:value="hostsRaw" @validate="onValidateHosts" />
      </NCard>

      <div class="flex items-center justify-between">
        <span class="text-13px" :class="saveHint ? 'text-warning' : 'text-gray-400'">{{ saveHint }}</span>
        <NSpace :size="8">
          <NButton @click="handleCancel">取消</NButton>
          <NButton type="primary" :loading="saving" :disabled="!canSave" @click="handleSave">保存修改</NButton>
        </NSpace>
      </div>
    </NSpace>
  </NSpin>
</template>

<style scoped></style>
