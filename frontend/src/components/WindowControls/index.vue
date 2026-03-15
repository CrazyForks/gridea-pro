<template>
  <div v-if="showControls" class="window-controls" style="--wails-draggable: no-drag">
    <!-- 最小化 -->
    <button class="control-btn" @click="minimize" :title="t('window.minimize')">
      <svg width="10" height="1" viewBox="0 0 10 1">
        <rect fill="currentColor" width="10" height="1" />
      </svg>
    </button>
    <!-- 最大化/还原 -->
    <button class="control-btn" @click="toggleMaximize" :title="t('window.zoom')">
      <svg v-if="!isMaximized" width="10" height="10" viewBox="0 0 10 10">
        <rect fill="none" stroke="currentColor" stroke-width="1" x="0.5" y="0.5" width="9" height="9" />
      </svg>
      <svg v-else width="10" height="10" viewBox="0 0 10 10">
        <rect fill="none" stroke="currentColor" stroke-width="1" x="2.5" y="0.5" width="7" height="7" />
        <rect fill="none" stroke="currentColor" stroke-width="1" x="0.5" y="2.5" width="7" height="7" />
      </svg>
    </button>
    <!-- 关闭 -->
    <button class="control-btn close-btn" @click="close" :title="t('window.close')">
      <svg width="10" height="10" viewBox="0 0 10 10">
        <line stroke="currentColor" stroke-width="1.2" x1="0" y1="0" x2="10" y2="10" />
        <line stroke="currentColor" stroke-width="1.2" x1="10" y1="0" x2="0" y2="10" />
      </svg>
    </button>
  </div>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Environment,
  WindowMinimise,
  WindowToggleMaximise,
  Quit,
  EventsOn,
} from '@/wailsjs/runtime'

const { t } = useI18n()
const showControls = ref(false)
const isMaximized = ref(false)

onMounted(async () => {
  try {
    const env = await Environment()
    showControls.value = env.platform !== 'darwin'
    if (showControls.value) {
      document.documentElement.classList.add('platform-frameless')
    }
  } catch {
    showControls.value = false
  }

  EventsOn('wails:window-maximised', () => {
    isMaximized.value = true
  })
  EventsOn('wails:window-restored', () => {
    isMaximized.value = false
  })
})

const minimize = () => WindowMinimise()
const toggleMaximize = () => WindowToggleMaximise()
const close = () => Quit()
</script>

<style scoped>
.window-controls {
  position: fixed;
  top: 0;
  right: 0;
  z-index: 9999;
  display: flex;
  height: 48px;
  -webkit-app-region: no-drag;
}

.control-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 46px;
  height: 48px;
  border: none;
  background: transparent;
  color: var(--foreground, #333);
  cursor: pointer;
  transition: background-color 0.15s ease;
  outline: none;
}

.control-btn:hover {
  background-color: hsl(var(--foreground) / 0.1);
}

.control-btn:active {
  background-color: hsl(var(--foreground) / 0.15);
}

.close-btn:hover {
  background-color: #e81123;
  color: white;
}

.close-btn:active {
  background-color: #bf0f1d;
  color: white;
}
</style>
