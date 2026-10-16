<script setup lang="ts">
import { h, onMounted, ref } from 'vue';
import type { DataTableColumns, DataTableRowKey, FormInst, FormRules } from 'naive-ui';
import { NButton, NPopconfirm, NSpace, NTag, NTooltip } from 'naive-ui';
import {
  fetchDeleteKickstart,
  fetchGetKickstartContent,
  fetchGetKickstarts,
  fetchUpdateKickstart,
  fetchUploadKickstart
} from '@/service/api';
import { getApiErrorCode } from '@/service/api/errors';
import { kickstartArchOptions } from '@/constants/isobuilder';
import { EMPTY_VALUE, formatBytes, formatDateTime } from '@/utils/format';
import { useClipboard } from '@vueuse/core';

const loading = ref(false);
const kickstarts = ref<Api.IsoBuilder.Kickstart[]>([]);

const previewShow = ref(false);
const previewLoading = ref(false);
const previewName = ref('');
const previewContent = ref('');

const uploadShow = ref(false);
const uploadSubmitting = ref(false);
const uploadFormRef = ref<FormInst | null>(null);
const uploadInputRef = ref<HTMLInputElement | null>(null);
const uploadFile = ref<File | null>(null);
const uploadForm = ref({
  name: '',
  arch: 'any' as Api.IsoBuilder.KickstartArch
});

const uploadRules: FormRules = {
  name: { required: true, message: '请输入模板名称', trigger: ['input', 'blur'] }
};

/** the name derived from the selected file, it follows the file until the user edits the name */
let derivedName = '';

async function loadKickstarts() {
  loading.value = true;

  const { data, error } = await fetchGetKickstarts();

  if (!error && data) {
    kickstarts.value = data.kickstarts;
  }

  loading.value = false;
}

async function handlePreview(row: Api.IsoBuilder.Kickstart) {
  previewName.value = row.name;
  previewContent.value = '';
  previewShow.value = true;
  previewLoading.value = true;

  const { data, error } = await fetchGetKickstartContent(row.id);

  previewLoading.value = false;

  if (error || !data) return;

  previewContent.value = data.content;
}

/** 编辑抽屉的状态：编辑内置模板时正文只读，元数据可改 */
const editShow = ref(false);
const editSubmitting = ref(false);
const editTarget = ref<Api.IsoBuilder.Kickstart | null>(null);
const editForm = ref({ name: '', remark: '', content: '' });
const editLoading = ref(false);
const editContentTouched = ref(false);

function handleEdit(row: Api.IsoBuilder.Kickstart) {
  editTarget.value = row;
  editForm.value = { name: row.name, remark: row.remark, content: '' };
  editContentTouched.value = false;
  editShow.value = true;
  editLoading.value = true;

  // 打开抽屉时拉一份当前正文作为编辑初值
  fetchGetKickstartContent(row.id).then(({ data, error }) => {
    editLoading.value = false;

    if (error || !data) {
      window.$message?.error('加载模板正文失败');
      return;
    }

    editForm.value.content = data.content;
  });
}

async function handleEditSubmit() {
  if (!editTarget.value) return;

  editSubmitting.value = true;

  const payload: Api.IsoBuilder.KickstartUpdatePayload = {
    name: editForm.value.name.trim() || undefined,
    remark: editForm.value.remark
  };

  // 正文只在编辑器改动过时才提交，避免覆盖别人的修改
  if (editContentTouched.value) {
    payload.content = editForm.value.content;
  }

  const { data, error } = await fetchUpdateKickstart(editTarget.value.id, payload);

  editSubmitting.value = false;

  if (error || !data) return;

  window.$message?.success('模板已更新');
  editShow.value = false;
  await loadKickstarts();
}

async function copyPreview() {
  if (!previewContent.value) return;

  const { copy, isSupported } = useClipboard({ legacy: true });

  if (!isSupported.value) {
    window.$message?.warning('当前浏览器不支持剪贴板，请手动复制。');
    return;
  }

  await copy(previewContent.value);
  window.$message?.success('已复制到剪贴板');
}

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const selected = input.files?.[0] ?? null;

  // reset the input so that the same file can be picked twice in a row
  input.value = '';

  if (!selected) return;

  const nextDerived = selected.name.replace(/\.[^.]+$/, '');

  if (!uploadForm.value.name.trim() || uploadForm.value.name === derivedName) {
    uploadForm.value.name = nextDerived;
  }

  derivedName = nextDerived;
  uploadFile.value = selected;
}

function resetUploadForm() {
  uploadFile.value = null;
  derivedName = '';
  uploadForm.value = { name: '', arch: 'any' };
}

async function handleUpload() {
  const selected = uploadFile.value;

  if (!selected) {
    window.$message?.warning('请先选择要上传的 .ks 模板文件。');
    return;
  }

  try {
    await uploadFormRef.value?.validate();
  } catch {
    return;
  }

  uploadSubmitting.value = true;

  const { error } = await fetchUploadKickstart({
    file: selected,
    name: uploadForm.value.name.trim() || undefined,
    arch: uploadForm.value.arch
  });

  uploadSubmitting.value = false;

  if (error) return;

  window.$message?.success('模板已上传');
  uploadShow.value = false;
  resetUploadForm();
  await loadKickstarts();
}

async function handleDelete(row: Api.IsoBuilder.Kickstart) {
  const { error } = await fetchDeleteKickstart(row.id);

  if (error) {
    if (getApiErrorCode(error) === 'kickstart_in_use') {
      window.$message?.warning('模板已被项目引用，无法删除。');
    }

    return;
  }

  window.$message?.success('模板已删除');
  await loadKickstarts();
}

const columns: DataTableColumns<Api.IsoBuilder.Kickstart> = [
  {
    key: 'name',
    title: '名称',
    minWidth: 220,
    render: row =>
      h('div', { class: 'min-w-0 py-4px' }, [
        h('span', { class: 'truncate text-15px font-500', title: row.name }, row.name)
      ])
  },
  {
    key: 'arch',
    title: '架构',
    width: 110,
    render: row => h('span', { class: 'font-mono' }, row.arch)
  },
  {
    key: 'source',
    title: '来源',
    width: 100,
    render: row =>
      h(
        NTag,
        { size: 'small', bordered: false, type: row.builtin ? 'info' : 'default' },
        { default: () => (row.builtin ? '内置' : '上传') }
      )
  },
  {
    key: 'remark',
    title: '备注',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render: row => row.remark || h('span', { class: 'text-gray-400' }, EMPTY_VALUE)
  },
  {
    key: 'size_bytes',
    title: '大小',
    width: 110,
    render: row => h('span', { class: 'font-mono' }, formatBytes(row.size_bytes))
  },
  {
    key: 'created_at',
    title: '创建时间',
    width: 170,
    render: row => h('span', { class: 'font-mono text-13px' }, formatDateTime(row.created_at))
  },
  {
    key: 'actions',
    title: '操作',
    width: 130,
    fixed: 'right',
    align: 'center',
    render: row => {
      const removeButton = h(
        NButton,
        { size: 'small', text: true, type: 'error', disabled: row.builtin },
        { default: () => '删除' }
      );

      // a disabled button swallows the pointer events, so the tooltip needs a wrapper
      const remove = row.builtin
        ? h(
            NTooltip,
            { trigger: 'hover' },
            {
              trigger: () => h('span', { class: 'inline-flex' }, [removeButton]),
              default: () => '内置模板不可删除'
            }
          )
        : h(
            NPopconfirm,
            { positiveText: '删除', negativeText: '取消', onPositiveClick: () => handleDelete(row) },
            {
              trigger: () => removeButton,
              default: () => `删除模板「${row.name}」？`
            }
          );

      return h(NSpace, { size: 8, justify: 'center', wrap: false }, {
        default: () => [
          h(
            NButton,
            { size: 'small', text: true, type: 'primary', onClick: () => handlePreview(row) },
            { default: () => '预览' }
          ),
          h(
            NButton,
            { size: 'small', text: true, type: 'primary', onClick: () => handleEdit(row) },
            { default: () => '编辑' }
          ),
          remove
        ]
      });
    }
  }
];

const rowKey = (row: Api.IsoBuilder.Kickstart): DataTableRowKey => row.id;

onMounted(() => {
  loadKickstarts();
});
</script>

<template>
  <div class="flex-col-stretch gap-16px">
    <NCard :bordered="false" class="card-wrapper" size="medium">
    <div class="mb-16px flex flex-wrap items-center justify-between gap-12px">
      <div>
        <h2 class="text-18px font-500">Kickstart 模板库</h2>
        <p class="mt-4px text-13px text-gray-500">内置与自定义的应答模板，构建时注入到原版镜像中完成无人值守安装。</p>
      </div>
      <NSpace :size="8" :wrap="true">
        <NButton :loading="loading" @click="loadKickstarts()">
          <template #icon>
            <SvgIcon icon="mdi:refresh" />
          </template>
          刷新
        </NButton>
        <NButton type="primary" @click="uploadShow = true">
          <template #icon>
            <SvgIcon icon="mdi:upload" />
          </template>
          上传模板
        </NButton>
      </NSpace>
    </div>

    <NDataTable
      :columns="columns"
      :data="kickstarts"
      :loading="loading"
      :row-key="rowKey"
      :scroll-x="1000"
      size="medium"
      bordered
      :single-line="false"
    >
      <template #empty>
        <div class="py-40px">
          <NEmpty description="还没有模板，上传一个 .ks 文件试试" />
        </div>
      </template>
    </NDataTable>

    <NDrawer v-model:show="previewShow" placement="right" width="min(100vw, 720px)">
      <NDrawerContent :title="`模板预览 · ${previewName}`" :native-scrollbar="false" closable>
        <template #footer>
          <NButton type="primary" :disabled="!previewContent" @click="copyPreview">复制全文</NButton>
        </template>
        <NSpin :show="previewLoading">
          <pre class="m-0 max-h-[calc(100vh-180px)] overflow-y-auto whitespace-pre-wrap rounded-6px bg-gray-50 p-12px font-mono text-12px dark:bg-gray-900">{{ previewContent || (previewLoading ? '' : '模板正文为空') }}</pre>
        </NSpin>
      </NDrawerContent>
    </NDrawer>

    <NDrawer v-model:show="editShow" placement="right" width="min(100vw, 720px)">
      <NDrawerContent :title="`编辑模板 · ${editTarget?.name ?? ''}`" :native-scrollbar="false" closable>
        <NSpin :show="editLoading">
          <div class="flex flex-col gap-16px">
            <NForm label-placement="top" :show-require-mark="false">
              <NFormItem label="名称">
                <NInput v-model:value="editForm.name" :maxlength="80" />
              </NFormItem>
              <NFormItem label="备注">
                <NInput v-model:value="editForm.remark" type="textarea" :rows="2" :maxlength="200" show-count />
              </NFormItem>
              <NFormItem label="模板正文">
                <NInput
                  v-if="editTarget?.builtin"
                  :value="editForm.content"
                  type="textarea"
                  :rows="16"
                  readonly
                  class="font-mono"
                />
                <NInput
                  v-else
                  :value="editForm.content"
                  type="textarea"
                  :rows="16"
                  class="font-mono"
                  @update:value="v => { editForm.content = v; editContentTouched = true; }"
                />
              </NFormItem>
            </NForm>
            <NAlert v-if="editTarget?.builtin" type="info" :bordered="false">
              内置模板正文不可编辑，但名称/备注可改。需要修改内容时复制一份上传为新模板。
            </NAlert>
          </div>
        </NSpin>

        <template #footer>
          <NSpace justify="end">
            <NButton @click="editShow = false">取消</NButton>
            <NButton type="primary" :loading="editSubmitting" @click="handleEditSubmit">保存</NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>

    <NDrawer v-model:show="uploadShow" placement="right" width="min(100vw, 480px)">
      <NDrawerContent title="上传 Kickstart 模板" :native-scrollbar="false" closable>
        <div class="flex flex-col gap-12px">
          <input ref="uploadInputRef" type="file" accept=".ks" class="hidden" @change="handleFileChange" />

          <div class="flex flex-wrap items-center gap-8px">
            <NButton @click="uploadInputRef?.click()">
              <template #icon>
                <SvgIcon icon="mdi:folder-open-outline" />
              </template>
              选择 .ks 文件
            </NButton>
            <NButton v-if="uploadFile" size="small" text type="error" @click="resetUploadForm">移除</NButton>
          </div>

          <div v-if="uploadFile" class="rounded-6px bg-gray-50 px-12px py-8px dark:bg-gray-900">
            <div class="break-all text-13px">{{ uploadFile.name }}</div>
            <div class="font-mono text-12px text-gray-400">{{ formatBytes(uploadFile.size) }}</div>
          </div>

          <NForm
            ref="uploadFormRef"
            :model="uploadForm"
            :rules="uploadRules"
            label-placement="top"
            :show-require-mark="false"
          >
            <NFormItem label="模板名称" path="name">
              <NInput v-model:value="uploadForm.name" placeholder="例如 rocky9-auto.ks" />
            </NFormItem>
            <NFormItem label="架构" path="arch">
              <NSelect v-model:value="uploadForm.arch" :options="kickstartArchOptions" />
            </NFormItem>
          </NForm>
        </div>

        <template #footer>
          <NSpace justify="end">
            <NButton @click="uploadShow = false">取消</NButton>
            <NButton type="primary" :loading="uploadSubmitting" @click="handleUpload">上传</NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>
    </NCard>
  </div>
</template>

<style scoped></style>
