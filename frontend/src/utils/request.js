import axios from 'axios'
import { getToken, removeToken } from './auth'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// Request interceptor: inject JWT
request.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor: centralize error handling
request.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const skipErrorMessage = error?.config?.skipErrorMessage === true

    if (error.response) {
      const { status, data, config } = error.response
      // 排除登录接口的 401（登录失败是正常业务逻辑）
      const isLoginApi = config.url === '/auth/login' || config.url === '/auth/register'
      if (status === 401 && !isLoginApi) {
        removeToken()
        window.location.href = '/login'
        return
      }
      if (!skipErrorMessage) {
        ElMessage.error(data?.message || 'Request failed')
      }
    } else if (!skipErrorMessage) {
      ElMessage.error('Network error, please check your connection')
    }

    return Promise.reject(error)
  }
)

export default request
