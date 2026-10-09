<script setup lang="ts">
import { computed, ref } from 'vue';
import { getPaletteColorByNumber, mixColor } from '@sa/color';
import { useAuthStore } from '@/store/modules/auth';
import { useConfigStore } from '@/store/modules/config';
import { useThemeStore } from '@/store/modules/theme';
import { useRouterPush } from '@/hooks/common/router';

const authStore = useAuthStore();
const configStore = useConfigStore();
const themeStore = useThemeStore();
const { routerPushByKey } = useRouterPush();

const token = ref('');
const errorMsg = ref('');

const authRequired = computed(() => configStore.authRequired);

const bgThemeColor = computed(() =>
  themeStore.darkMode ? getPaletteColorByNumber(themeStore.themeColor, 600) : themeStore.themeColor
);

const bgColor = computed(() => mixColor('#ffffff', themeStore.themeColor, themeStore.darkMode ? 0.5 : 0.2));

async function handleLogin() {
  errorMsg.value = '';

  const value = token.value.trim();

  if (!value) {
    errorMsg.value = '请输入访问令牌';
    return;
  }

  const pass = await authStore.login(value);

  if (!pass) {
    errorMsg.value = '令牌校验失败，请确认与后端 ISOBUILDER_AUTH_TOKEN 一致。';
  }
}
</script>

<template>
  <div class="relative size-full flex-center overflow-hidden" :style="{ backgroundColor: bgColor }">
    <WaveBg :theme-color="bgThemeColor" />
    <NCard :bordered="false" class="relative z-4 w-auto rd-12px">
      <div class="w-400px lt-sm:w-300px">
        <header class="flex-y-center justify-between">
          <SystemLogo class="size-64px lt-sm:size-48px" />
          <h3 class="text-28px text-primary font-500 lt-sm:text-22px">ISO Builder</h3>
          <ThemeSchemaSwitch
            :theme-schema="themeStore.themeScheme"
            :show-tooltip="false"
            class="text-20px lt-sm:text-18px"
            @switch="themeStore.toggleThemeScheme"
          />
        </header>
        <main class="pt-24px">
          <h3 class="text-18px text-primary font-medium">访问认证</h3>

          <NAlert v-if="!authRequired" class="mt-16px" type="info" :bordered="false">
            后端未开启令牌认证，可直接进入控制台。
          </NAlert>

          <template v-else>
            <p class="mt-12px text-14px text-gray-500">
              后端已开启认证，请输入与 <code class="font-mono">ISOBUILDER_AUTH_TOKEN</code> 一致的访问令牌。
            </p>
            <div class="pt-20px">
              <NInput
                v-model:value="token"
                type="password"
                show-password-on="click"
                placeholder="访问令牌"
                :input-props="{ autocomplete: 'off' }"
                @keyup.enter="handleLogin"
              >
                <template #prefix>
                  <SvgIcon icon="mdi:key-variant" />
                </template>
              </NInput>
            </div>
            <NAlert v-if="errorMsg" class="mt-12px" type="error" :bordered="false">{{ errorMsg }}</NAlert>
            <NButton class="mt-24px w-full" type="primary" :loading="authStore.loginLoading" @click="handleLogin">
              进入控制台
            </NButton>
          </template>

          <NButton
            v-if="!authRequired"
            class="mt-24px w-full"
            type="primary"
            @click="routerPushByKey('root')"
          >
            进入控制台
          </NButton>
        </main>
      </div>
    </NCard>
  </div>
</template>

<style scoped></style>
