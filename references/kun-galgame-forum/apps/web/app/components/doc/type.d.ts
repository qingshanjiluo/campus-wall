import type { DocArticleDetail } from '~/types/doc'

export interface KunTreeNode {
  name: string
  label: string
  path: string
  children?: KunTreeNode[]
  type: 'file' | 'directory'
}

export type KunFrontmatter = DocArticleDetail
