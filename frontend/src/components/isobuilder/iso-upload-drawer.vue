<script setup lang="ts">
import { ref, watch } from 'vue';
import type { FormInst, FormRules } from 'naive-ui';
import { fetchRegisterIso } from '@/service/api';
import { getApiErrorCode, getApiErrorMessage } from '@/service/api/errors';
import { useRouterPush } from '@/hooks/common/router';
import { archOptions } from '@/constants/isobuilder';

interface Props {
  /** whether the drawer is visible */
  show: boolean;
  /** which way of adding an image the drawer opens on */
  mode?: 'url' | 'path';
}

const props = withDefaults(defineProps<Props>(), { mode: 'url' });

const emit = defineEmits<{
  'update:show': [value: boolean];
  finished: [];
}>();

const { routerPushByKey } = useRouterPush();

/**
 * How an image enters the library.
 *
 * The browser never uploads an image: a base ISO is tens of gigabytes, so the
 * server fetches it instead — either from an HTTP(S) link (object storage
 * presigned/public URLs included) or by adopting a file already on its disk.
 */
const activeTab = ref<'url' | 'path'>(props.mode);

watch(
  () => props.show,
  visible => {
    if (visible) {
      activeTab.value = props.mode;
      form.value = { name: '', arch: 'x86_64', url: '', path: '' };
      errorMessage.value = '';
      errorCode.value = '';
    }
  }
);

const formRef = ref<FormInst | null>(null);
const submitting = ref(false);
const errorMessage = ref('');
const errorCode = ref('');

const form = ref({
  name: '',
  arch: 'x86_64' as Api.IsoBuilder.Arch,
  url: '',
  path: ''
});

const rules: FormRules = {
  url: [{ required: true, message: '请输入下载链接', trigger: ['input', 'blur'] }],
  path: [{ required: true, message: '请输入服务器上的绝对路径', trigger: ['input', 'blur'] }]
};

function handleUpdateShow(value: boolean) {
  if (submitting.value) return;
  emit('update:show', value);
}

async function handleSubmit() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }

  submitting.value = true;
  errorMessage.value = '';
  errorCode.value = '';

  const isUrl = activeTab.value === 'url';
  const { error, data } = await fetchRegisterIso({
    source: isUrl ? 'url' : 'path',
    [isUrl ? 'url' : 'path']: isUrl ? form.value.url.trim() : form.value.path.trim(),
    name: form.value.name.trim() || undefined,
    arch: form.value.arch
  });

  submitting.value = false;

  if (error) {
    errorMessage.value = getApiErrorMessage(error) || (isUrl ? '提交失败' : '注册失败');
    errorCode.value = getApiErrorCode(error) ?? '';
    return;
  }

  if (data) {
    emit('finished');
    emit('update:show', false);
  }
}

function goSettings() {
  emit('update:show', false);
  routerPushByKey('settings');
}
</script>

<template>
  <NDrawer
    :show="show"
    placement="right"
    width="min(100vw, 520px)"
    :mask-closable="!submitting"
    :close-on-esc="!submitting"
    @update:show="handleUpdateShow"
  >
    <NDrawerContent title="添加基础镜像" :native-scrollbar="false" closable>
      <div class="mb-4">
        <NRadioGroup v-model:value="activeTab" size="small" :disabled="submitting">
          <NRadioButton value="url">链接导入</NRadioButton>
          <NRadioButton value="path">本地注册</NRadioButton>
        </NRadioGroup>
      </div>

      <NForm ref="formRef" :model="form" :rules="rules" label-placement="top" :show-require-mark="false">
        <NFormItem label="镜像名称" path="name">
          <NInput v-model:value="form.name" :disabled="submitting" placeholder="留空则取链接/文件名" />
        </NFormItem>

        <NFormItem label="架构" path="arch">
          <NSelect v-model:value="form.arch" :options="archOptions" :disabled="submitting" />
        </NFormItem>

        <NFormItem v-if="activeTab === 'url'" label="下载链接" path="url">
          <NInput
            v-model:value="form.url"
            :disabled="submitting"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="https://mirrors.example.com/Rocky-9.8-x86_64-dvd.iso"
          />
        </NFormItem>

        <NFormItem v-else label="服务器上的绝对路径" path="path">
          <NInput
            v-model:value="form.path"
            :disabled="submitting"
            :input-props="{ class: 'font-mono', spellcheck: 'false', autocomplete: 'off' }"
            placeholder="/var/lib/isobuilder/data/isos/Rocky-9.8-x86_64-dvd.iso"
          />
        </NFormItem>
      </NForm>

      <NAlert type="default" :bordered="false" :show-icon="false">
        <template v-if="activeTab === 'url'">
          服务端会先探活链接，通过后在后台流式下载并计算 SHA256。对象存储的公开链接或预签名链接同样适用。
        </template>
        <template v-else>
          文件必须已在服务端数据目录内。服务端直接读取并计算 SHA256，不复制文件。
        </template>
      </NAlert>

      <NAlert v-if="errorMessage" class="mt-4" type="error" :bordered="false" :show-icon="true" title="提交失败">
        <div class="break-all">{{ errorMessage }}</div>
        <NButton
          v-if="errorCode === 'storage_not_configured'"
          class="mt-2"
          size="tiny"
          secondary
          type="primary"
          @click="goSettings"
        >
          前往「系统配置」填写凭据
        </NButton>
      </NAlert>

      <template #footer>
        <NSpace justify="end">
          <NButton :disabled="submitting" @click="handleUpdateShow(false)">关闭</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSubmit">
            {{ activeTab === 'url' ? '开始下载' : '注册' }}
          </NButton>
        </NSpace>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped></style>
