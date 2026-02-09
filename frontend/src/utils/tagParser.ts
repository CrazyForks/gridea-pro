/**
 * 从文本内容中解析 #标签
 * @param content 文本内容
 * @returns 标签数组（不含 # 符号）
 */
export function parseTags(content: string): string[] {
    const regex = /#([\p{L}\p{N}_]+)/gu
    const matches = content.matchAll(regex)
    const tagSet = new Set<string>()

    for (const match of matches) {
        if (match[1]) {
            tagSet.add(match[1])
        }
    }

    return Array.from(tagSet)
}

/**
 * 高亮文本中的 #标签
 * @param content 文本内容
 * @param tagClass CSS 类名
 * @returns 带有 HTML 标签的高亮文本
 */
export function highlightTags(content: string, tagClass = 'text-primary'): string {
    return content.replace(
        /#([\p{L}\p{N}_]+)/gu,
        `<span class="${tagClass}">#$1</span>`
    )
}
