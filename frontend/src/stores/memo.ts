import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { IMemo, IMemoStats, IMemoLoadResponse, IMemoSaveResponse } from '@/interfaces/memo'
import { EventsEmit, EventsOnce, EventsOff } from 'wailsjs/runtime'

export const useMemoStore = defineStore('memo', () => {
    // State
    const memos = ref<IMemo[]>([])
    const stats = ref<IMemoStats | null>(null)
    const loading = ref(false)
    const selectedTag = ref<string | null>(null)
    const selectedDate = ref<string | null>(null)
    const timeFilter = ref<'all' | 'today' | 'month'>('all')
    const keyword = ref('')

    // Getters
    const filteredMemos = computed(() => {
        let result = memos.value

        // Keyword Filter
        if (keyword.value) {
            const lowerKeyword = keyword.value.toLowerCase()
            result = result.filter(memo => memo.content.toLowerCase().includes(lowerKeyword))
        }

        // Time Filter
        if (selectedDate.value) {
            const targetDate = selectedDate.value // YYYY-MM-DD
            result = result.filter(m => {
                const mDate = new Date(m.createdAt)
                const year = mDate.getFullYear()
                const month = String(mDate.getMonth() + 1).padStart(2, '0')
                const day = String(mDate.getDate()).padStart(2, '0')
                return `${year}-${month}-${day}` === targetDate
            })
        } else if (timeFilter.value === 'today') {
            const now = new Date()
            const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
            result = result.filter(m => new Date(m.createdAt).getTime() >= startOfDay)
        } else if (timeFilter.value === 'month') {
            const now = new Date()
            const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1).getTime()
            result = result.filter(m => new Date(m.createdAt).getTime() >= startOfMonth)
        }


        if (selectedTag.value) {
            result = result.filter(memo => memo.tags?.includes(selectedTag.value!))
        }
        return result
    })

    const totalMemos = computed(() => stats.value?.total || memos.value.length)
    const todayMemos = computed(() => {
        const now = new Date()
        const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
        return memos.value.filter(m => new Date(m.createdAt).getTime() >= startOfDay).length
    })
    const monthMemos = computed(() => {
        const now = new Date()
        const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1).getTime()
        return memos.value.filter(m => new Date(m.createdAt).getTime() >= startOfMonth).length
    })
    const totalTags = computed(() => stats.value?.tags?.length || 0)
    const heatmapData = computed(() => stats.value?.heatmap || {})
    const tagStats = computed(() => stats.value?.tags || [])

    // Actions
    function fetchMemos(): Promise<void> {
        return new Promise((resolve, reject) => {
            loading.value = true
            EventsEmit('memo-load')
            EventsOnce('memo-loaded', (result: IMemoLoadResponse) => {
                loading.value = false
                if (result.success) {
                    memos.value = result.memos || []
                    stats.value = result.stats
                    resolve()
                } else {
                    reject(new Error('Failed to load memos'))
                }
            })
        })
    }

    function saveMemo(content: string): Promise<IMemo | undefined> {
        return new Promise((resolve, reject) => {
            EventsEmit('memo-save', content)
            EventsOnce('memo-saved', (result: IMemoSaveResponse) => {
                if (result.success) {
                    memos.value = result.memos || []
                    stats.value = result.stats
                    resolve(result.memo)
                } else {
                    reject(new Error('Failed to save memo'))
                }
            })
        })
    }

    function updateMemo(memo: IMemo): Promise<void> {
        return new Promise((resolve, reject) => {
            EventsEmit('memo-update', memo)
            EventsOnce('memo-updated', (result: IMemoSaveResponse) => {
                if (result.success) {
                    memos.value = result.memos || []
                    stats.value = result.stats
                    resolve()
                } else {
                    reject(new Error('Failed to update memo'))
                }
            })
        })
    }

    function deleteMemo(id: string): Promise<void> {
        return new Promise((resolve, reject) => {
            EventsEmit('memo-delete', id)
            EventsOnce('memo-deleted', (result: IMemoSaveResponse) => {
                if (result.success) {
                    memos.value = result.memos || []
                    stats.value = result.stats
                    resolve()
                } else {
                    reject(new Error('Failed to delete memo'))
                }
            })
        })
    }

    function renameTag(oldName: string, newName: string): Promise<void> {
        return new Promise((resolve, reject) => {
            EventsEmit('memo-rename-tag', oldName, newName)
            EventsOnce('memo-renamed-tag', (result: IMemoSaveResponse) => {
                if (result.success) {
                    memos.value = result.memos || []
                    stats.value = result.stats
                    resolve()
                } else {
                    reject(new Error('Failed to rename tag'))
                }
            })
        })
    }

    function deleteTag(tagName: string): Promise<void> {
        return new Promise((resolve, reject) => {
            EventsEmit('memo-delete-tag', tagName)
            EventsOnce('memo-deleted-tag', (result: IMemoSaveResponse) => {
                if (result.success) {
                    memos.value = result.memos || []
                    stats.value = result.stats
                    resolve()
                } else {
                    reject(new Error('Failed to delete tag'))
                }
            })
        })
    }

    function setTimeFilter(filter: 'all' | 'today' | 'month') {
        timeFilter.value = filter
        if (filter !== 'all') {
            selectedTag.value = null
            selectedDate.value = null
        }
    }

    function setSelectedTag(tag: string | null) {
        selectedTag.value = tag
        if (tag) {
            timeFilter.value = 'all'
            selectedDate.value = null
        }
    }

    function setSelectedDate(date: string | null) {
        selectedDate.value = date
        if (date) {
            timeFilter.value = 'all'
            selectedTag.value = null
        }
    }

    function cleanup() {
        EventsOff('memo-loaded')
        EventsOff('memo-saved')
        EventsOff('memo-updated')
        EventsOff('memo-deleted')
        EventsOff('memo-renamed-tag')
        EventsOff('memo-deleted-tag')
    }

    return {
        // State
        memos,
        stats,
        loading,
        selectedTag,
        selectedDate,
        timeFilter,
        keyword,
        // Getters
        filteredMemos,
        totalMemos,
        todayMemos,
        monthMemos,
        totalTags,
        heatmapData,
        tagStats,
        // Actions
        fetchMemos,
        saveMemo,
        updateMemo,
        deleteMemo,
        renameTag,
        deleteTag,
        setSelectedTag,
        setSelectedDate,
        setTimeFilter,
        cleanup,
    }
})
