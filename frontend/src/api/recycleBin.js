import request from '../utils/request'

export function listRecycleBin(params) {
  return request.get('/recycle-bin', { params })
}

export function restoreItem(id) {
  return request.post(`/recycle-bin/${id}/restore`)
}

export function permanentDelete(id) {
  return request.delete(`/recycle-bin/${id}`)
}

export function emptyRecycleBin() {
  return request.post('/recycle-bin/empty')
}
