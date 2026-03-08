<template>
  <div class="pb-20 max-w-4xl mx-auto pt-4">
    <div class="space-y-6">
      <!-- 启用 CDN -->
      <div class="grid grid-cols-[180px_1fr] items-center gap-4">
        <label class="text-sm font-medium text-right text-muted-foreground">{{ t('settings.cdn.enable') }}</label>
        <div class="flex items-center gap-3">
          <Switch :checked="form.enabled" @update:checked="(v: boolean) => form.enabled = v" />
          <span class="text-xs text-muted-foreground">{{ t('settings.cdn.enableDesc') }}</span>
        </div>
      </div>

      <template v-if="form.enabled">
        <!-- 提示信息 -->
        <div class="grid grid-cols-[180px_1fr] items-start gap-4">
          <div></div>
          <div class="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-3 text-xs text-amber-700 dark:text-amber-400 max-w-sm">
            {{ t('settings.cdn.notice') }}
          </div>
        </div>

        <!-- CDN 服务商 -->
        <div class="grid grid-cols-[180px_1fr] items-center gap-4">
          <label class="text-sm font-medium text-right text-muted-foreground">{{ t('settings.cdn.provider') }}</label>
          <div class="w-full max-w-sm">
            <Select :model-value="form.provider" @update:model-value="(v) => form.provider = v as string">
              <SelectTrigger>
                <SelectValue :placeholder="t('settings.cdn.provider')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="jsdelivr">jsDelivr</SelectItem>
                <SelectItem value="custom">{{ t('settings.cdn.custom') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <!-- jsDelivr 配置 -->
        <template v-if="form.provider === 'jsdelivr'">
          <div class="grid grid-cols-[180px_1fr] items-center gap-4">
            <label class="text-sm font-medium text-right text-muted-foreground">{{ t('settings.cdn.githubUser') }}</label>
            <div class="max-w-sm">
              <Input v-model="form.githubUser" placeholder="username" />
            </div>
          </div>
          <div class="grid grid-cols-[180px_1fr] items-center gap-4">
            <label class="text-sm font-medium text-right text-muted-foreground">{{ t('settings.cdn.githubRepo') }}</label>
            <div class="max-w-sm">
              <Input v-model="form.githubRepo" placeholder="repo-name" />
            </div>
          </div>
          <div class="grid grid-cols-[180px_1fr] items-center gap-4">
            <label class="text-sm font-medium text-right text-muted-foreground">{{ t('settings.cdn.githubBranch') }}</label>
            <div class="max-w-sm">
              <Input v-model="form.githubBranch" placeholder="main" />
            </div>
          </div>
          <div class="grid grid-cols-[180px_1fr] items-center gap-4">
            <div></div>
            <div class="text-xs text-muted-foreground">
              {{ t('settings.cdn.jsdelivrTip') }}<br>
              <code class="text-primary/80">cdn.jsdelivr.net/gh/{{ form.githubUser || 'user' }}/{{ form.githubRepo || 'repo' }}@{{ form.githubBranch || 'main' }}/</code>
            </div>
          </div>
        </template>

        <!-- 自定义 CDN -->
        <template v-if="form.provider === 'custom'">
          <div class="grid grid-cols-[180px_1fr] items-center gap-4">
            <label class="text-sm font-medium text-right text-muted-foreground">{{ t('settings.cdn.baseUrl') }}</label>
            <div class="max-w-sm">
              <Input v-model="form.baseUrl" placeholder="https://cdn.example.com" />
              <div class="text-xs text-muted-foreground mt-1.5">{{ t('settings.cdn.baseUrlDesc') }}</div>
            </div>
          </div>
        </template>
      </template>
    </div>

    <footer-box>
      <div class="flex justify-end items-center w-full">
        <Button
          variant="default"
          class="w-18 h-8 text-xs justify-center rounded-full bg-primary text-background hover:bg-primary/90 cursor-pointer"
          @click="submit">
          {{ t('common.save') }}
        </Button>
      </div>
    </footer-box>
  </div>
</template>

<script lang="ts" setup>
import { reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from '@/helpers/toast'
import FooterBox from '@/components/FooterBox/index.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { GetCdnSetting, SaveCdnSettingFromFrontend } from '@/wailsjs/go/facade/CdnSettingFacade'
import { domain } from '@/wailsjs/go/models'

const { t } = useI18n()

const form = reactive({
  enabled: false,
  provider: 'jsdelivr',
  githubUser: '',
  githubRepo: '',
  githubBranch: 'main',
  baseUrl: '',
})

onMounted(async () => {
  try {
    const setting = await GetCdnSetting()
    if (setting) {
      form.enabled = setting.enabled || false
      form.provider = setting.provider || 'jsdelivr'
      form.githubUser = setting.githubUser || ''
      form.githubRepo = setting.githubRepo || ''
      form.githubBranch = setting.githubBranch || 'main'
      form.baseUrl = setting.baseUrl || ''
    }
  } catch (e) {
    console.error('Failed to load CDN settings', e)
  }
})

const submit = async () => {
  try {
    const settingDomain = new domain.CdnSetting(form)
    await SaveCdnSettingFromFrontend(settingDomain)
    toast.success(t('settings.cdn.saveSuccess'))
  } catch (e) {
    console.error(e)
    toast.error(t('settings.cdn.saveFailed'))
  }
}
</script>
