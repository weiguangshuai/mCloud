<template>
  <div class="login-page">
    <div class="login-bg-pattern"></div>
    <div class="login-main">
      <div class="login-brand">
        <div class="brand-icon">
          <el-icon :size="32"><Cloudy /></el-icon>
        </div>
        <h1 class="brand-title">mCloud</h1>
        <p class="brand-subtitle">私人网盘</p>
      </div>

      <div class="login-card">
        <h2 class="card-title">创建账号</h2>
        <p class="card-desc">开始使用私人网盘</p>

        <el-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleRegister">
          <el-form-item prop="username">
            <el-input
              v-model="form.username"
              placeholder="用户名"
              size="large"
              :prefix-icon="User"
            />
          </el-form-item>
          <el-form-item prop="nickname">
            <el-input
              v-model="form.nickname"
              placeholder="昵称（可选）"
              size="large"
              :prefix-icon="UserFilled"
            />
          </el-form-item>
          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="密码"
              size="large"
              :prefix-icon="Lock"
              show-password
            />
          </el-form-item>
          <el-form-item prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              placeholder="确认密码"
              size="large"
              :prefix-icon="Lock"
              show-password
              @keyup.enter="handleRegister"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              size="large"
              class="login-btn"
              :loading="loading"
              @click="handleRegister"
            >
              注 册
            </el-button>
          </el-form-item>
        </el-form>

        <div class="login-footer">
          已有账号？<router-link to="/login">去登录</router-link>
        </div>
      </div>

      <p class="login-copyright">mCloud &copy; 2024</p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Cloudy, User, UserFilled, Lock } from '@element-plus/icons-vue'
import { register } from '../api/auth'
import { setToken } from '../utils/auth'
import { useUserStore } from '../store'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref(null)
const loading = ref(false)

const form = reactive({ username: '', nickname: '', password: '', confirmPassword: '' })

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== form.password) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '用户名长度 3-50 个字符', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 个字符', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

async function handleRegister() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await register({
      username: form.username,
      password: form.password,
      nickname: form.nickname,
    })
    setToken(res.data.token)
    userStore.setUser(res.data.user)
    ElMessage.success('注册并登录成功')
    router.push('/')
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '注册失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-primary);
  position: relative;
  overflow: hidden;
}

.login-bg-pattern {
  position: absolute;
  inset: 0;
  opacity: 0.4;
  background-image:
    radial-gradient(circle at 20% 30%, rgba(45, 55, 72, 0.03) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(45, 55, 72, 0.02) 0%, transparent 40%);
}

.login-main {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 32px;
  padding: 24px;
}

.login-brand {
  text-align: center;
}

.brand-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-hover) 100%);
  border-radius: 20px;
  margin-bottom: 16px;
  box-shadow: 0 8px 24px rgba(45, 55, 72, 0.15);
}

.brand-icon .el-icon {
  color: #FFFFFF;
}

.brand-title {
  font-family: var(--font-title);
  font-size: 28px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 4px;
  letter-spacing: 2px;
}

.brand-subtitle {
  font-size: 14px;
  color: var(--text-muted);
  letter-spacing: 1px;
}

.login-card {
  width: 360px;
  max-width: 90vw;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 40px 36px;
  box-shadow: var(--shadow-soft);
}

.card-title {
  font-family: var(--font-title);
  font-size: 22px;
  font-weight: 500;
  color: var(--text-primary);
  text-align: center;
  margin-bottom: 8px;
}

.card-desc {
  font-size: 14px;
  color: var(--text-muted);
  text-align: center;
  margin-bottom: 32px;
}

.login-card :deep(.el-form-item) {
  margin-bottom: 20px;
}

.login-card :deep(.el-input__wrapper) {
  padding: 4px 12px;
}

.login-card :deep(.el-input__inner) {
  font-size: 15px;
}

.login-btn {
  width: 100%;
  height: 48px;
  font-family: var(--font-title);
  font-size: 15px;
  font-weight: 500;
  letter-spacing: 4px;
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-hover) 100%) !important;
  border: none !important;
  border-radius: var(--radius-md) !important;
  transition: all var(--transition-normal) !important;
  box-shadow: 0 4px 12px rgba(45, 55, 72, 0.2) !important;
}

.login-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(45, 55, 72, 0.25) !important;
}

.login-btn:active {
  transform: translateY(0);
}

.login-footer {
  text-align: center;
  font-size: 14px;
  color: var(--text-muted);
  margin-top: 8px;
}

.login-footer a {
  color: var(--accent-primary);
  font-weight: 500;
  transition: color var(--transition-fast);
}

.login-footer a:hover {
  color: var(--accent-hover);
}

.login-copyright {
  font-size: 12px;
  color: var(--text-muted);
  letter-spacing: 0.5px;
}
</style>
