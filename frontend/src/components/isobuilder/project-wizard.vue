<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import type { SelectOption } from 'naive-ui';
import {
  fetchCreateProject,
  fetchGetIsos,
  fetchGetKickstartContent,
  fetchGetKickstarts,
  fetchGetScripts,
} from '@/service/api';
import { getApiErrorCode, getApiErrorMessage } from '@/service/api/errors';
import { archOptions, isoSourceRecord } from '@/constants/isobuilder';
import { useRouterPush } from '@/hooks/common/router';
import { useAppStore } from '@/store/modules/app';
import { formatBytes } from '@/utils/format';

const { routerPushByKey } = useRouterPush();
const appStore = useAppStore();

const steps = [
  { key: 'basic', title: '基础信息', description: '项目名称与目标架构' },
  { key: 'iso', title: '基础镜像', description: '选用一个已就绪的 ISO' },
  { key: 'kickstart', title: '应答模板', description: '选用 kickstart 模板' },
  { key: 'hosts', title: '主机清单', description: '粘贴并校验主机信息' },
  { key: 'post-script', title: '自定义脚本', description: '安装完成后执行的自定义脚本' },
  { key: 'advanced', title: '高级项', description: 'ISO 卷标与产物文件名' }
] as const;

const currentStep = ref(0);
const currentStepInfo = computed(() => steps[currentStep.value] || steps[0]);

const drawerWidth = computed(() => (appStore.isMobile ? '100%' : 720));

const saving = ref(false);

const form = ref({
  name: '',
  description: '',
  arch: 'x86_64' as Api.IsoBuilder.Arch,
  iso_id: '',
  kickstart_id: '',
  iso_label: '',
  output_name: '',
  payload_mode: 'off' as Api.IsoBuilder.PayloadMode
});
const payloadModeOptions: SelectOption[] = [
  { label: '不注入（默认）', value: 'off' },
  { label: '按卷标查找 U 盘分区', value: 'label' },
  { label: '卷标优先，未命中则扫描', value: 'auto' }
];
const hostsRaw = ref('');
const hostsResult = ref<Api.IsoBuilder.ValidateHostsResult | null>(null);
const postScript = ref('');
/** 脚本库（可复用）；selectedScriptId 为本项目选中的条目 */
const scripts = ref<Api.IsoBuilder.Script[]>([]);
const selectedScriptId = ref('');
const scriptOptions = computed(() => scripts.value.map(item => ({ label: item.name, value: item.id })));

const isos = ref<Api.IsoBuilder.Iso[]>([]);
const kickstarts = ref<Api.IsoBuilder.Kickstart[]>([]);
const loadingIsos = ref(false);
const loadingKickstarts = ref(false);

/** the first render waits for the iso and kickstart catalogues */
const spinLoading = computed(() => loadingIsos.value || loadingKickstarts.value);
const showAllArch = ref(false);

const selectedIso = computed(() => isos.value.find(iso => iso.id === form.value.iso_id) || null);

/** the archived iso of the project has been deleted from the library */
const selectedIsoMissing = computed(
  () => Boolean(form.value.iso_id) && !loadingIsos.value && !isos.value.some(iso => iso.id === form.value.iso_id)
);

/** the isos of the target arch, the currently selected one is always kept visible */
const visibleIsos = computed(() => {
  if (showAllArch.value) return isos.value;

  return isos.value.filter(iso => iso.arch === form.value.arch || iso.id === form.value.iso_id);
});

const canSelectIso = (iso: Api.IsoBuilder.Iso) => iso.status === 'ready';

const canSelectKickstart = (kickstart: Api.IsoBuilder.Kickstart) =>
  kickstart.arch === 'any' || kickstart.arch === form.value.arch;

/** 后端唯一认的 U 盘注入替换点（builder/pipeline.go 里写死）。 */
const PAYLOAD_PLACEHOLDER = '%%PAYLOAD_SNIPPET%%';

/**
 * 后端会把 %%任意大写%% 都当成占位符列出来，但真正替换的只有两个名字。
 * 其余的原样留在 kickstart 里，装机时静默失效 —— 这种笔误值得当场提示。
 */
const KNOWN_PLACEHOLDERS: Record<string, true> = {
  '%%HOSTS_JSON_PLACEHOLDER%%': true,
  [PAYLOAD_PLACEHOLDER]: true
};

const unknownPlaceholders = (kickstart: Api.IsoBuilder.Kickstart) =>
  (kickstart.placeholders ?? []).filter(placeholder => !KNOWN_PLACEHOLDERS[placeholder]);

const hostHint = computed(() => {
  if (!hostsRaw.value.trim()) return '请填写主机清单。';

  if (!hostsResult.value) return '主机清单尚未校验通过，请等待自动校验完成后重试。';

  if (!hostsResult.value.ok) return `主机清单存在 ${hostsResult.value.errors.length} 处问题，请修正后再继续。`;

  return '';
});

const canGoNext = computed(() => {
  switch (currentStep.value) {
    case 0:
      return Boolean(form.value.name.trim());
    case 1:
      return Boolean(form.value.iso_id) && (!selectedIso.value || canSelectIso(selectedIso.value));
    case 2:
      return Boolean(form.value.kickstart_id);
    case 3:
      return !hostHint.value;
    default:
      return true;
  }
});

/** 只要填了名称就能存草稿；完整性由构建时把关 */
const canSaveDraft = computed(() => Boolean(form.value.name.trim()));

/** 完整保存（创建后可直接构建）要求三项齐全且清单校验通过 */
const canSave = computed(
  () => Boolean(form.value.name.trim()) && Boolean(form.value.iso_id) && Boolean(form.value.kickstart_id) && !hostHint.value
);

/** the reason why the user cannot move on from the current step */
const stepHint = computed(() => {
  switch (currentStep.value) {
    case 0:
      return form.value.name.trim() ? '' : '请填写项目名称。';
    case 1:
      return form.value.iso_id ? '' : '请选择一个状态为「可用」的基础镜像。';
    case 2:
      return form.value.kickstart_id ? '' : '请选择一个 kickstart 模板。';
    case 3:
      return hostHint.value;
    default:
      return '';
  }
});

function isoUnavailableReason(iso: Api.IsoBuilder.Iso) {
  switch (iso.status) {
    case 'downloading':
      return '镜像仍在下载中，下载完成并校验通过后才能选用。';
    case 'hashing':
      return '正在计算 SHA256，校验通过后才能选用。';
    case 'error':
      return `镜像异常：${iso.error || '原因未知'}`;
    default:
      return '';
  }
}

function selectIso(iso: Api.IsoBuilder.Iso) {
  if (!canSelectIso(iso)) return;

  form.value.iso_id = iso.id;
}

function selectKickstart(kickstart: Api.IsoBuilder.Kickstart) {
  if (!canSelectKickstart(kickstart)) return;

  form.value.kickstart_id = kickstart.id;
}

function onValidateHosts(result: Api.IsoBuilder.ValidateHostsResult | null) {
  hostsResult.value = result;
}

function handlePrev() {
  if (currentStep.value > 0) currentStep.value -= 1;
}

function handleNext() {
  if (!canGoNext.value || currentStep.value >= steps.length - 1) return;

  currentStep.value += 1;
}

function handleCancel() {
  routerPushByKey('project-list');
}

async function loadIsos() {
  loadingIsos.value = true;
  const { data, error } = await fetchGetIsos();
  loadingIsos.value = false;

  if (!error) isos.value = data.isos;
}

async function loadKickstarts() {
  loadingKickstarts.value = true;
  const { data, error } = await fetchGetKickstarts();
  loadingKickstarts.value = false;

  if (!error) kickstarts.value = data.kickstarts;
}

async function loadScripts() {
  const { data, error } = await fetchGetScripts();

  if (!error) scripts.value = data.scripts;
}

async function handleSave(draft = false) {
  if (draft ? !canSaveDraft.value : !canSave.value) return;

  saving.value = true;

  const payload: Api.IsoBuilder.ProjectPayload = {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    arch: form.value.arch,
    iso_id: form.value.iso_id,
    kickstart_id: form.value.kickstart_id,
    iso_label: form.value.iso_label.trim(),
    output_name: form.value.output_name.trim(),
    post_script: postScript.value,
    script_id: selectedScriptId.value,
    payload_mode: form.value.payload_mode,
    hosts_raw: hostsRaw.value,
    // 格式交给后端自动识别，编辑器不再让用户选
    hosts_format: 'auto'
  };

  const { data, error } = await fetchCreateProject(payload);

  saving.value = false;

  if (error) return;

  window.$message?.success(draft ? '已保存草稿，随时可以回来继续' : '项目已创建');
  routerPushByKey('project-detail', { params: { id: data.project.id } });
}

/** the kickstart preview drawer */
const previewVisible = ref(false);
const previewKickstart = ref<Api.IsoBuilder.Kickstart | null>(null);
const previewContent = ref('');
const loadingPreview = ref(false);

async function openPreview(kickstart: Api.IsoBuilder.Kickstart) {
  previewKickstart.value = kickstart;
  previewContent.value = '';
  previewVisible.value = true;
  loadingPreview.value = true;

  const { data, error } = await fetchGetKickstartContent(kickstart.id);

  loadingPreview.value = false;

  if (error) return;

  previewContent.value = data.content;
}

const isoSelectOptions = computed(() =>
  isos.value
    .filter(iso => iso.status === 'ready')
    .map(iso => ({ label: `${iso.name} · ${iso.arch}`, value: iso.id }))
);

const kickstartSelectOptions = computed(() =>
  kickstarts.value.map(kickstart => ({ label: `${kickstart.name} · ${kickstart.arch}`, value: kickstart.id }))
);


onMounted(async () => {
  await Promise.all([loadIsos(), loadKickstarts(), loadScripts()]);
});
</script>

<template>
  <NSpin :show="spinLoading">
    <div class="flex flex-col gap-16px">
      <div class="flex flex-wrap items-center justify-between gap-12px">
        <div>
          <div class="text-16px font-500">第 {{ currentStep + 1 }} / {{ steps.length }} 步 · {{ currentStepInfo.title }}</div>
          <div class="mt-4px text-13px text-gray-500">{{ currentStepInfo.description }}</div>
        </div>
        <NSpace :size="8">
          <NButton :loading="saving" :disabled="!canSaveDraft" @click="handleSave(true)">
            <template #icon>
              <SvgIcon icon="mdi:content-save-outline" />
            </template>
            保存草稿
          </NButton>
          <NButton @click="handleCancel">取消</NButton>
        </NSpace>
      </div>

      <div class="overflow-x-auto pb-4px">
        <NSteps :current="currentStep + 1" size="small" style="min-width: 620px">
          <NStep v-for="step in steps" :key="step.key" :title="step.title" :description="step.description" />
        </NSteps>
      </div>

      <!-- 1. 基础信息 -->
      <NForm v-if="currentStep === 0" label-placement="top">
        <NFormItem label="项目名称" required>
          <NInput v-model:value="form.name" placeholder="例如：京西机房 perf 批次" maxlength="100" show-count />
        </NFormItem>
        <NFormItem label="描述">
          <NInput
            v-model:value="form.description"
            type="textarea"
            :rows="3"
            placeholder="选填，说明这批主机的用途、机房或批次"
            maxlength="500"
          />
        </NFormItem>
        <NFormItem label="目标架构" required>
          <NRadioGroup v-model:value="form.arch">
            <NRadioButton v-for="option in archOptions" :key="String(option.value)" :value="option.value">
              {{ option.label }}
            </NRadioButton>
          </NRadioGroup>
        </NFormItem>
      </NForm>

      <!-- 2. 基础镜像 -->
      <div v-else-if="currentStep === 1" class="flex flex-col gap-12px">
        <div class="flex flex-wrap items-center justify-between gap-8px">
          <span class="text-14px text-gray-500">
            仅状态为「可用」的镜像可以被选用，其它状态会置灰并说明原因。
          </span>
          <NCheckbox v-model:checked="showAllArch">显示全部架构</NCheckbox>
        </div>

        <NAlert v-if="selectedIsoMissing" type="warning" size="small" :bordered="false">
          当前项目绑定的基础镜像已从镜像库删除，请重新选择一个镜像。
        </NAlert>

        <div v-if="loadingIsos" class="py-24px text-center">
          <NSpin size="small" />
          <div class="mt-8px text-13px text-gray-500">正在加载镜像库…</div>
        </div>

        <NEmpty v-else-if="!isos.length" description="镜像库中还没有基础镜像，请先上传或登记一个 ISO。">
          <template #extra>
            <NButton type="primary" @click="routerPushByKey('iso-library')">去镜像库上传</NButton>
          </template>
        </NEmpty>

        <NEmpty v-else-if="!visibleIsos.length" :description="`镜像库中没有 ${form.arch} 架构的镜像。`">
          <template #extra>
            <NSpace :size="8">
              <NButton @click="showAllArch = true">显示全部架构</NButton>
              <NButton type="primary" @click="routerPushByKey('iso-library')">去镜像库上传</NButton>
            </NSpace>
          </template>
        </NEmpty>

        <div v-else class="grid grid-cols-1 gap-12px md:grid-cols-2 xl:grid-cols-3">
          <div
            v-for="iso in visibleIsos"
            :key="iso.id"
            class="select-card"
            :class="{
              'select-card--selected': iso.id === form.iso_id,
              'select-card--disabled': !canSelectIso(iso)
            }"
            role="radio"
            :aria-checked="iso.id === form.iso_id"
            :aria-disabled="!canSelectIso(iso)"
            @click="selectIso(iso)"
          >
            <div class="flex items-start justify-between gap-8px">
              <div class="min-w-0">
                <div class="truncate text-14px font-500">{{ iso.name }}</div>
                <div class="truncate text-12px font-mono text-gray-500">{{ iso.filename }}</div>
              </div>
              <IsoStatusTag :status="iso.status" :animated="false" />
            </div>
            <div class="flex flex-wrap items-center gap-8px text-12px text-gray-500">
              <span class="font-mono">{{ iso.arch }}</span>
              <span class="font-mono">{{ formatBytes(iso.size_bytes) }}</span>
              <span>{{ isoSourceRecord[iso.source].label }}</span>
            </div>
            <div v-if="!canSelectIso(iso)" class="text-12px text-warning">
              {{ isoUnavailableReason(iso) }}
            </div>
          </div>
        </div>

      </div>

      <!-- 3. kickstart 模板 -->
      <div v-else-if="currentStep === 2" class="flex flex-col gap-12px">
        <span class="text-14px text-gray-500">
          装机时 kickstart 会被写入 ISO，其中的占位符由构建阶段替换为主机清单。
        </span>

        <div v-if="loadingKickstarts" class="py-24px text-center">
          <NSpin size="small" />
          <div class="mt-8px text-13px text-gray-500">正在加载模板库…</div>
        </div>

        <NEmpty v-else-if="!kickstarts.length" description="模板库中还没有 kickstart 模板。">
          <template #extra>
            <NButton type="primary" @click="routerPushByKey('kickstart-library')">去模板库上传</NButton>
          </template>
        </NEmpty>

        <div v-else class="grid grid-cols-1 gap-12px md:grid-cols-2">
          <div
            v-for="kickstart in kickstarts"
            :key="kickstart.id"
            class="select-card"
            :class="{
              'select-card--selected': kickstart.id === form.kickstart_id,
              'select-card--disabled': !canSelectKickstart(kickstart)
            }"
            role="radio"
            :aria-checked="kickstart.id === form.kickstart_id"
            :aria-disabled="!canSelectKickstart(kickstart)"
            @click="selectKickstart(kickstart)"
          >
            <div class="flex items-start justify-between gap-8px">
              <div class="min-w-0">
                <div class="truncate text-14px font-500">{{ kickstart.name }}</div>
                <div class="mt-4px flex flex-wrap items-center gap-6px text-12px text-gray-500">
                  <NTag size="tiny" :bordered="false" class="font-mono">{{ kickstart.arch }}</NTag>
                  <NTag v-if="kickstart.builtin" size="tiny" type="info" :bordered="false">内置</NTag>
                  <span class="font-mono">{{ formatBytes(kickstart.size_bytes) }}</span>
                </div>
              </div>
              <NButton size="tiny" quaternary @click.stop="openPreview(kickstart)">
                <template #icon>
                  <SvgIcon icon="mdi:file-eye-outline" />
                </template>
                预览
              </NButton>
            </div>

            <!--
              只展示有信息量的能力，不再平铺占位符原文：
              %%HOSTS_JSON_PLACEHOLDER%% 是准入门槛（缺了上传就被拒、构建也会失败），
              所有可用模板都有，展示它没有区分度。真正要看的是模板里有没有
              %%PAYLOAD_SNIPPET%% —— 有替换点，项目的 payload_mode 才会生效。
            -->
            <div class="flex flex-wrap items-center gap-6px">
              <NTag
                v-if="kickstart.placeholders.includes(PAYLOAD_PLACEHOLDER)"
                size="tiny"
                type="info"
                :bordered="false"
              >
                支持 U 盘注入
              </NTag>
              <NTooltip v-if="unknownPlaceholders(kickstart).length">
                <template #trigger>
                  <NTag size="tiny" type="error" :bordered="false">
                    {{ unknownPlaceholders(kickstart).length }} 个未知占位符
                  </NTag>
                </template>
                这些占位符构建时不会被替换，会原样留在 kickstart 里：{{
                  unknownPlaceholders(kickstart).join('、')
                }}
              </NTooltip>
            </div>

            <div v-if="!canSelectKickstart(kickstart)" class="text-12px text-warning">
              该模板仅适用于 {{ kickstart.arch }} 架构。
            </div>
          </div>
        </div>
      </div>

      <!-- 4. 主机清单 -->
      <div v-else-if="currentStep === 3" class="flex flex-col gap-12px">
        <HostsEditor v-model:value="hostsRaw" @validate="onValidateHosts" />
      </div>

      <!-- 5. 自定义脚本 -->
      <div v-else-if="currentStep === 4" class="flex flex-col gap-12px">
        <NForm label-placement="top" :show-feedback="false">
          <NFormItem label="脚本库（可选，可复用）">
            <NSelect
              v-model:value="selectedScriptId"
              :options="scriptOptions"
              clearable
              placeholder="从脚本库选一个可复用脚本；也可只在下方写本项目专属脚本"
            />
          </NFormItem>
          <NFormItem label="从 U 盘注入文件（可选）">
            <NSelect v-model:value="form.payload_mode" :options="payloadModeOptions" />
          </NFormItem>
          <NFormItem label="本项目追加脚本（可选）">
            <NInput
              v-model:value="postScript"
              type="textarea"
              :rows="10"
              placeholder="# 留空则不追加。库脚本先执行，这里的内容排在其后。"
              class="font-mono"
            />
          </NFormItem>
        </NForm>
        <NAlert type="info" :bordered="false" :show-icon="true">
          <div class="flex flex-col gap-4px text-13px">
            <span>脚本在系统安装完成后的 kickstart <span class="font-mono">%post</span> 阶段执行，位于网络配置之后。</span>
            <span>执行顺序：<span class="font-500">先跑脚本库里选中的脚本，再跑本项目的追加脚本</span>。</span>
            <span>内容以 base64 注入，无需转义特殊字符；执行记录在 <span class="font-mono">/var/log/isobuilder/</span>。</span>
          </div>
        </NAlert>
      </div>

      <!-- 6. 高级项 -->
      <NForm v-else label-placement="top">
        <NFormItem label="ISO 卷标（iso_label）">
          <NInput v-model:value="form.iso_label" placeholder="留空则构建时自动探测原版 ISO 的卷标" />
        </NFormItem>
        <NFormItem label="产物文件名（output_name）">
          <NInput v-model:value="form.output_name" placeholder="留空则构建时自动生成，例如 xxx-auto.iso" />
        </NFormItem>
        <NAlert type="info" :bordered="false" :show-icon="true">
          <div class="flex flex-col gap-4px text-13px">
            <span>
              <span class="font-mono">iso_label</span>
              留空时，构建阶段会挂载原版 ISO 并自动探测其卷标（volume label）后使用。
            </span>
            <span>
              <span class="font-mono">output_name</span>
              留空时，构建阶段会按项目与架构自动生成产物文件名；填写时建议以
              <span class="font-mono">.iso</span> 结尾。
            </span>
          </div>
        </NAlert>
      </NForm>

      <NAlert v-if="stepHint" type="warning" size="small" :bordered="false" :show-icon="true">
        {{ stepHint }}
      </NAlert>

      <div class="flex flex-wrap items-center justify-between gap-12px">
        <NButton :disabled="currentStep === 0" @click="handlePrev">
          <template #icon>
            <SvgIcon icon="mdi:chevron-left" />
          </template>
          上一步
        </NButton>
        <NButton v-if="currentStep < steps.length - 1" type="primary" :disabled="!canGoNext" @click="handleNext">
          下一步
          <template #icon>
            <SvgIcon icon="mdi:chevron-right" />
          </template>
        </NButton>
        <NButton v-else type="primary" :loading="saving" :disabled="!canSave" @click="handleSave()">
          <template #icon>
            <SvgIcon icon="mdi:check-circle-outline" />
          </template>
          创建项目
        </NButton>
      </div>
    </div>
  </NSpin>

  <NDrawer v-model:show="previewVisible" :width="drawerWidth" placement="right">
    <NDrawerContent :title="`预览：${previewKickstart?.name || ''}`" :native-scrollbar="false" closable>
      <NSpin :show="loadingPreview">
        <div class="min-h-240px">
          <pre v-if="previewContent" class="kickstart-preview font-mono text-12px">{{ previewContent }}</pre>
          <NEmpty v-else-if="!loadingPreview" description="模板内容加载失败" />
        </div>
      </NSpin>
    </NDrawerContent>
  </NDrawer>

</template>

<style lang="scss" scoped>
.select-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid rgb(var(--gray-300-color));
  border-radius: 8px;
  cursor: pointer;
  transition:
    border-color 0.2s,
    background-color 0.2s;
}

.select-card:hover {
  border-color: rgb(var(--primary-color));
}

.select-card--selected {
  border-color: rgb(var(--primary-color));
  background-color: rgb(var(--primary-color) / 6%);
}

.select-card--disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.select-card--disabled:hover {
  border-color: rgb(var(--gray-300-color));
}

.kickstart-preview {
  margin: 0;
  padding: 12px;
  overflow-x: auto;
  border: 1px solid rgb(var(--gray-300-color));
  border-radius: 6px;
  background-color: rgb(var(--container-bg-color));
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}
</style>
