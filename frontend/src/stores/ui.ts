import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * UI 状态 store — 解耦 scheduling.ts 与 app.ts 的循环依赖
 * 存放跨组件通信的临时 UI 状态（对话框触发等）
 */
export const useUiStore = defineStore('ui', () => {
  // 排课完成后触发导航确认对话框
  const pendingScheduleNav = ref(false)
  // 排课失败后触发错误对话框
  const pendingScheduleError = ref('')

  function clearScheduleNav() { pendingScheduleNav.value = false }
  function clearScheduleError() { pendingScheduleError.value = '' }

  return {
    pendingScheduleNav,
    pendingScheduleError,
    clearScheduleNav,
    clearScheduleError,
  }
})
