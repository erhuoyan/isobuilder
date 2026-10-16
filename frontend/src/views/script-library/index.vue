<script setup lang="ts">
import { h, onMounted, ref } from 'vue';
import type { DataTableColumns, DataTableRowKey, FormInst, FormRules } from 'naive-ui';
import { NButton, NPopconfirm, NSpace } from 'naive-ui';
import {
  fetchCreateScript,
  fetchDeleteScript,
  fetchGetScripts,
  fetchUpdateScript
} from '@/service/api';
import { getApiErrorCode } from '@/service/api/errors';
import { EMPTY_VALUE, formatBytes, formatDateTime } from '@/utils/format';
import { useClipboard } from '@vueuse/core';

const loading = ref(false);
const scripts = ref<Api.IsoBuilder.Script[]>([]);

/** 预览抽屉 */
const previewShow = ref(false);
const previewName = ref('');
const previewContent = ref('');

/** 新建 / 编辑抽屉：editTarget 为 null 表示新建 */
const editShow = ref(false);
const editSubmitting = ref(false);
const editTarget = ref<Api.IsoBuilder.Script | null>(null);
const editFormRef = ref<FormInst | null>(null);
const editForm = ref({ name: '', remark: '', content: '' });

const editRules: FormRules = {
  name: { required: true, message: '请输入脚本名称', trigger: ['input', 'blur'] },
  content: { required: true, message: '请输入脚本内容', trigger: ['input', 'blur'] }
};

async function loadScripts() {
  loading.value = true;

  const { data, error } = await fetchGetScripts();

  if (!error && data) {
    scripts.value = data.scripts;
  }

  loading.value = false;
}

function openPreview(row: Api.IsoBuilder.Script) {
  previewName.value = row.name;
  previewContent.value = row.content;
  previewShow.value = true;
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

function openCreate() {
  editTarget.value = null;
  editForm.value = { name: '', remark: '', content: '' };
  editShow.value = true;
}

function openEdit(row: Api.IsoBuilder.Script) {
  editTarget.value = row;
  editForm.value = { name: row.name, remark: row.remark, content: row.content };
  editShow.value = true;
}

async function handleSubmit() {
  try {
    await editFormRef.value?.validate();
  } catch {
    return;
  }

  editSubmitting.value = true;

  const payload = {
    name: editForm.value.name.trim(),
    remark: editForm.value.remark.trim(),
    content: editForm.value.content
  };

  const target = editTarget.value;
  const { error } = target
    ? await fetchUpdateScript(target.id, payload)
    : await fetchCreateScript(payload);

  editSubmitting.value = false;

  if (error) {
    if (getApiErrorCode(error) === 'too_large') {
      window.$message?.error('脚本超过 64 KiB 上限。');
    }

    return;
  }

  window.$message?.success(target ? '脚本已更新' : '脚本已创建');
  editShow.value = false;
  await loadScripts();
}

async function handleDelete(row: Api.IsoBuilder.Script) {
  const { error } = await fetchDeleteScript(row.id);

  if (error) {
    if (getApiErrorCode(error) === 'script_in_use') {
      window.$message?.warning('该脚本已被项目引用，请先在那些项目里解除绑定。');
    }

    return;
  }

  window.$message?.success('脚本已删除');
  await loadScripts();
}

const columns: DataTableColumns<Api.IsoBuilder.Script> = [
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
    key: 'updated_at',
    title: '更新时间',
    width: 170,
    render: row => h('span', { class: 'font-mono text-13px' }, formatDateTime(row.updated_at))
  },
  {
    key: 'actions',
    title: '操作',
    width: 150,
    fixed: 'right',
    align: 'center',
    render: row =>
      h(NSpace, { size: 8, justify: 'center', wrap: false }, {
        default: () => [
          h(
            NButton,
            { size: 'small', text: true, type: 'primary', onClick: () => openPreview(row) },
            { default: () => '预览' }
          ),
          h(
            NButton,
            { size: 'small', text: true, type: 'primary', onClick: () => openEdit(row) },
            { default: () => '编辑' }
          ),
          h(
            NPopconfirm,
            { positiveText: '删除', negativeText: '取消', onPositiveClick: () => handleDelete(row) },
            {
              trigger: () =>
                h(NButton, { size: 'small', text: true, type: 'error' }, { default: () => '删除' }),
              default: () => `删除脚本「${row.name}」？被项目引用时无法删除。`
            }
          )
        ]
      })
  }
];

const rowKey = (row: Api.IsoBuilder.Script): DataTableRowKey => row.id;

onMounted(() => {
  loadScripts();
});
</script>

<template>
  <div class="flex-col-stretch gap-16px">
    <NCard :bordered="false" class="card-wrapper" size="medium">
      <div class="mb-16px flex flex-wrap items-center justify-between gap-12px">
        <div>
          <h2 class="text-18px font-500">自定义脚本</h2>
          <p class="mt-4px text-13px text-gray-500">
            可复用的安装后脚本库。项目里选中后在系统安装完成后执行；脚本库在前、项目内的追加脚本在后。
          </p>
        </div>
        <NSpace :size="8" :wrap="true">
          <NButton :loading="loading" @click="loadScripts()">
            <template #icon>
              <SvgIcon icon="mdi:refresh" />
            </template>
            刷新
          </NButton>
          <NButton type="primary" @click="openCreate">
            <template #icon>
              <SvgIcon icon="mdi:plus" />
            </template>
            新建脚本
          </NButton>
        </NSpace>
      </div>

      <NDataTable
        :columns="columns"
        :data="scripts"
        :loading="loading"
        :row-key="rowKey"
        :scroll-x="900"
        size="medium"
        bordered
        :single-line="false"
      >
        <template #empty>
          <div class="py-40px">
            <NEmpty description="脚本库还没有内容，点击「新建脚本」添加一个可复用的自定义脚本" />
          </div>
        </template>
      </NDataTable>
    </NCard>

    <NDrawer v-model:show="previewShow" placement="right" width="min(100vw, 720px)">
      <NDrawerContent :title="`预览 · ${previewName}`" :native-scrollbar="false" closable>
        <template #footer>
          <NButton type="primary" @click="copyPreview">复制内容</NButton>
        </template>
        <pre
          v-if="previewContent"
          class="m-0 rounded-6px border border-gray-300 bg-white p-12px font-mono text-12px leading-1.6"
          style="white-space: pre-wrap; word-break: break-all"
        >{{ previewContent }}</pre>
        <NEmpty v-else description="脚本内容为空" />
      </NDrawerContent>
    </NDrawer>

    <NDrawer v-model:show="editShow" placement="right" width="min(100vw, 720px)">
      <NDrawerContent
        :title="editTarget ? `编辑脚本 · ${editTarget.name}` : '新建脚本'"
        :native-scrollbar="false"
        closable
      >
        <NForm ref="editFormRef" :model="editForm" :rules="editRules" label-placement="top">
          <NFormItem label="名称" path="name">
            <NInput v-model:value="editForm.name" placeholder="例如：安装监控 agent（所有机器基线）" maxlength="100" />
          </NFormItem>
          <NFormItem label="备注">
            <NInput v-model:value="editForm.remark" placeholder="选填，说明适用场景" maxlength="200" />
          </NFormItem>
          <NFormItem label="脚本内容" path="content">
            <NInput
              v-model:value="editForm.content"
              type="textarea"
              :rows="16"
              class="font-mono"
              placeholder="#!/bin/bash&#10;yum install -y htop&#10;systemctl enable --now htop"
            />
          </NFormItem>
          <NAlert type="info" :bordered="false" :show-icon="true">
            <div class="flex flex-col gap-4px text-13px">
              <span>脚本以 base64 注入 kickstart 的独立 <span class="font-mono">%post</span> 段，无需转义特殊字符。</span>
              <span>建议以 <span class="font-mono">#!/bin/bash</span> 开头；上限 64 KiB。</span>
              <span>执行记录与退出码落在装机机的 <span class="font-mono">/var/log/isobuilder/</span>。</span>
            </div>
          </NAlert>
        </NForm>
        <template #footer>
          <NSpace :size="8">
            <NButton @click="editShow = false">取消</NButton>
            <NButton type="primary" :loading="editSubmitting" @click="handleSubmit">
              {{ editTarget ? '保存修改' : '创建脚本' }}
            </NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped></style>
