<script setup lang="ts">
import { ref } from 'vue'
import { NSwitch, NButton, NInputNumber, NSelect, NSpace } from 'naive-ui'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()

const autoSave = ref(true)
const realtimeCheck = ref(true)
</script>

<template>
  <div class="settings-page">
    <h2 class="page-title">系统设置</h2>

    <div class="settings-section">
      <h3 class="section-title">基本设置</h3>

      <div class="setting-item">
        <div>
          <div class="setting-label">深色模式</div>
          <div class="setting-desc">切换系统界面为深色主题</div>
        </div>
        <n-switch :value="appStore.theme === 'dark'" @update:value="appStore.toggleTheme()" />
      </div>

      <div class="setting-item">
        <div>
          <div class="setting-label">自动保存</div>
          <div class="setting-desc">排课修改后自动保存到本地数据库</div>
        </div>
        <n-switch v-model:value="autoSave" />
      </div>

      <div class="setting-item">
        <div>
          <div class="setting-label">冲突实时检测</div>
          <div class="setting-desc">拖拽调课时实时检测并高亮冲突</div>
        </div>
        <n-switch v-model:value="realtimeCheck" />
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">排课参数</h3>

      <div class="setting-item">
        <div>
          <div class="setting-label">默认算法迭代次数</div>
          <div class="setting-desc">自动排课时的遗传算法迭代轮数</div>
        </div>
        <n-input-number :value="5000" size="small" style="width:120px" />
      </div>

      <div class="setting-item">
        <div>
          <div class="setting-label">每日最大课时</div>
          <div class="setting-desc">单个班级每天最多安排的课时数</div>
        </div>
        <n-input-number :value="8" size="small" style="width:120px" />
      </div>

      <div class="setting-item">
        <div>
          <div class="setting-label">教室缓冲时间</div>
          <div class="setting-desc">两节课之间教室的间隔分钟数</div>
        </div>
        <n-select
          :options="[{label:'0',value:0},{label:'10',value:10},{label:'15',value:15},{label:'20',value:20}]"
          :value="10"
          size="small"
          style="width:120px"
        />
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">数据管理</h3>

      <div class="setting-item">
        <div>
          <div class="setting-label">数据库位置</div>
          <div class="setting-desc">SQLite 数据库文件存储路径</div>
        </div>
        <span class="setting-value">~/scheduling/data.db</span>
      </div>

      <div class="setting-item">
        <div>
          <div class="setting-label">备份与恢复</div>
          <div class="setting-desc">导出或导入数据库备份文件</div>
        </div>
        <n-space>
          <n-button size="small">导出备份</n-button>
          <n-button size="small">导入</n-button>
        </n-space>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  max-width: 680px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 24px;
}

.settings-section {
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  padding: 20px;
  margin-bottom: 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 16px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--b3-border-color);
}

.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
}

.setting-item + .setting-item {
  border-top: 1px solid var(--b3-border-color);
}

.setting-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-background);
}

.setting-desc {
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
  margin-top: 2px;
}

.setting-value {
  font-size: 12px;
  color: var(--b3-theme-on-surface);
}
</style>
