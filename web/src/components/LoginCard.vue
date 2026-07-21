<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { login } from '../api'

const emit = defineEmits(['logged-in'])
const username = ref('')
const password = ref('')
const loading = ref(false)

async function handleSubmit() {
  if (!username.value || !password.value) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await login(username.value, password.value)
    emit('logged-in')
  } catch (e) {
    ElMessage.error(e.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-card shadow="never" class="login-card">
    <template #header><span>登录</span></template>
    <el-form @submit.prevent="handleSubmit">
      <el-form-item label="用户名">
        <el-input v-model="username" autocomplete="username" @keyup.enter="handleSubmit" />
      </el-form-item>
      <el-form-item label="密码">
        <el-input
          v-model="password"
          type="password"
          show-password
          autocomplete="current-password"
          @keyup.enter="handleSubmit"
        />
      </el-form-item>
      <div class="actions">
        <el-button type="primary" :loading="loading" @click="handleSubmit">登录</el-button>
      </div>
    </el-form>
  </el-card>
</template>

<style scoped>
.login-card {
  max-width: 380px;
  margin: 0 auto;
}
.actions {
  text-align: center;
}
</style>
