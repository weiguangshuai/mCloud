import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUserStore = defineStore('user', () => {
  const userInfo = ref(null)
  const currentFolderID = ref(0)
  const breadcrumbs = ref([{ id: 0, name: '根目录' }])

  function setUser(user) {
    userInfo.value = user
  }

  function clearUser() {
    userInfo.value = null
  }

  function setCurrentFolder(folderID, folderBreadcrumbs) {
    currentFolderID.value = folderID
    if (folderBreadcrumbs) {
      breadcrumbs.value = folderBreadcrumbs
    }
  }

  return { userInfo, currentFolderID, breadcrumbs, setUser, clearUser, setCurrentFolder }
})
