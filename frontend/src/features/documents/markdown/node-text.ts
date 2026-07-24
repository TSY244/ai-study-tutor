import { isValidElement, type ReactNode } from 'react'

/** 从 react-markdown 子树递归提取纯文本（代码内容只取文本，忽略任何注入元素）。 */
export function nodeText(node: ReactNode): string {
  if (node == null || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(nodeText).join('')
  if (isValidElement<{ children?: ReactNode }>(node)) return nodeText(node.props.children)
  return ''
}
