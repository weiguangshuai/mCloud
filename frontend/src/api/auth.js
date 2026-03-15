import request from '../utils/request'

export function register(data) {
  return request.post('/auth/register', data)
}

export function login(data, config) {
  return request.post('/auth/login', data, config)
}

export function getProfile() {
  return request.get('/auth/profile')
}
