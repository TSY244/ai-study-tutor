import GithubSlugger from 'github-slugger'

/**
 * 目录条目。id 与 rehype-slug 写入标题的 id 一致：两边用同一个
 * github-slugger 算法、按相同的文档顺序生成（重复标题自动追加 -1/-2）。
 */
export interface TocHeading {
  depth: number
  text: string
  id: string
}

/**
 * 去掉行内 Markdown 标记，逼近 rehype-slug 所见的标题纯文本。
 * 覆盖常见内联语法即可；个别极端写法（标题里套显示公式）允许目录链接失配降级。
 */
function plainHeadingText(raw: string): string {
  return raw
    .replace(/!\[([^\]]*)]\([^)]*\)/g, '$1') // 图片 -> alt 文本
    .replace(/\[([^\]]+)]\([^)]*\)/g, '$1') // 链接 -> 链接文本
    .replace(/`([^`]*)`/g, '$1') // 行内代码
    .replace(/\$([^$]*)\$/g, '$1') // 行内公式 -> 原始表达式
    .replace(/[*_~]/g, '') // 强调/删除线
    .replace(/\s+/g, ' ')
    .trim()
}

/**
 * 从 Markdown 源提取标题目录（h1–h6）。逐行扫描并跟踪代码围栏，
 * 代码块内的 `#` 注释不会被误当标题；ATX 闭合 # 会被剥掉。
 */
export function extractToc(markdown: string): TocHeading[] {
  const slugger = new GithubSlugger()
  const headings: TocHeading[] = []
  let fenceChar = ''
  let fenceLength = 0

  for (const line of markdown.split('\n')) {
    const fence = /^ {0,3}(`{3,}|~{3,})/.exec(line)
    if (fence) {
      const marker = fence[1]
      if (fenceChar === '') {
        fenceChar = marker[0]
        fenceLength = marker.length
      } else if (marker[0] === fenceChar && marker.length >= fenceLength) {
        fenceChar = ''
        fenceLength = 0
      }
      continue
    }
    if (fenceChar !== '') continue

    const match = /^ {0,3}(#{1,6})\s+(.+?)\s*$/.exec(line)
    if (!match) continue
    const text = plainHeadingText(match[2].replace(/\s+#+$/, ''))
    if (text.length === 0) continue
    headings.push({ depth: match[1].length, text, id: slugger.slug(text) })
  }
  return headings
}
