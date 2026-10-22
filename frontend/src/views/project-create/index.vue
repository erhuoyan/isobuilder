<script setup lang="ts"></script>

<template>
  <div class="flex-col-stretch gap-4">
    <NCard :bordered="false" class="card-wrapper" size="small">
      <h2 class="mb-4 text-18px font-500">新建项目</h2>
      <ProjectWizard />
    </NCard>
  </div>
</template>

<style scoped></style>
