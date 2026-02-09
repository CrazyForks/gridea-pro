/**
 * Memo 接口定义
 */
export interface IMemo {
    id: string
    content: string
    tags: string[]
    images: string[]
    createdAt: string
    updatedAt: string
}



/**
 * 标签统计
 */
export interface ITagStat {
    name: string
    count: number
}

/**
 * Memo 统计数据
 */
export interface IMemoStats {
    total: number
    tags: ITagStat[]
    heatmap: Record<string, number>
}

/**
 * Memo 加载响应
 */
export interface IMemoLoadResponse {
    success: boolean
    memos: IMemo[]
    stats: IMemoStats | null
}

/**
 * Memo 保存响应
 */
export interface IMemoSaveResponse {
    success: boolean
    memo?: IMemo
    memos: IMemo[]
    stats: IMemoStats | null
}
