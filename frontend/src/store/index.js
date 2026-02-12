import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUserStore = defineStore('user', () => {
  const userInfo = ref(null)
  const currentFolderID = ref(0)
  const breadcrumbs = ref([{ id: 0, name: '根目录' }])

  function setUser(user) {
    userInfo.value = user
    const rootId = user?.root_folder_id || 0
    if (currentFolderID.value === 0) {
      currentFolderID.value = rootId
      breadcrumbs.value = [{ id: rootId, name: '根目录' }]
    }
  }

  function clearUser() {
    userInfo.value = null
    currentFolderID.value = 0
    breadcrumbs.value = [{ id: 0, name: '根目录' }]
  }

  function setCurrentFolder(folderID, folderBreadcrumbs) {
    currentFolderID.value = folderID
    if (folderBreadcrumbs) {
      breadcrumbs.value = folderBreadcrumbs
    }
  }

  function resetToRoot() {
    const rootId = userInfo.value?.root_folder_id || 0
    currentFolderID.value = rootId
    breadcrumbs.value = [{ id: rootId, name: '根目录' }]
  }

  return {
    userInfo,
    currentFolderID,
    breadcrumbs,
    setUser,
    clearUser,
    setCurrentFolder,
    resetToRoot,
  }
})
