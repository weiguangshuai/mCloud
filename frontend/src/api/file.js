import request from '../utils/request'

export function listFiles(params) {
  return request.get('/files', { params })
}

export function uploadFile(formData, onProgress) {
  return request.post('/files/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: onProgress,
  })
}

export function initChunkedUpload(data) {
  return request.post('/files/upload/init', data)
}

export function uploadChunk(formData, onProgress) {
  return request.post('/files/upload/chunk', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: onProgress,
  })
}

export function completeUpload(data) {
  return request.post('/files/upload/complete', data)
}

export function getUploadStatus(uploadId) {
  return request.get(`/files/upload/status/${uploadId}`)
}

export function deleteFile(id) {
  return request.delete(`/files/${id}`)
}

export function renameFile(id, name) {
  return request.put(`/files/${id}/rename`, { name })
}

export function moveFile(id, folderId) {
  return request.put(`/files/${id}/move`, { folder_id: folderId })
}

export function batchDeleteFiles(fileIds) {
  return request.post('/files/batch/delete', { file_ids: fileIds })
}

export function batchMoveFiles(fileIds, folderId) {
  return request.post('/files/batch/move', { file_ids: fileIds, folder_id: folderId })
}

export function getStorageQuota() {
  return request.get('/user/storage/quota')
}

export function downloadFileBlob(fileId) {
  return request.get(`/files/${fileId}/download`, { responseType: 'blob' })
}

export function fetchThumbnailBlob(fileId) {
  return request.get(`/files/${fileId}/thumbnail`, { responseType: 'blob' })
}

export function fetchPreviewBlob(fileId) {
  return request.get(`/files/${fileId}/preview`, { responseType: 'blob' })
}
