import request from '../utils/request'

export function listFolders(parentId = 0) {
  return request.get('/folders', { params: { parent_id: parentId } })
}

export function createFolder(data) {
  return request.post('/folders', data)
}

export function renameFolder(id, data) {
  return request.put(`/folders/${id}`, data)
}

export function deleteFolder(id) {
  return request.delete(`/folders/${id}`)
}
