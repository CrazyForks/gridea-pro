export interface IPost {
  title: string
  date: string
  published: boolean
  hideInList: boolean
  tags?: string[]
  categories?: string[]
  feature: string
  isTop: boolean
  content: string
  fileName: string
}
