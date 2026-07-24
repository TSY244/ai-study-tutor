import type { HighlighterCore, LanguageRegistration } from 'shiki/core'

/**
 * 懒加载语法高亮（02-frontend-handoff §6.3/§10）：
 * - shiki core、引擎、主题与语法全部走动态 import，首个代码块出现时才
 *   开始加载，不进入路由 chunk，更不进入首屏 bundle；
 * - 语言白名单按需 loadLanguage；未知语言、超长代码块退回纯文本渲染，
 *   避免一次性高成本高亮拖慢长文；
 * - 任何加载/高亮异常都退回纯文本，代码内容绝不丢失。
 * - 使用 JS 正则引擎而非 Oniguruma WASM：无 wasm 加载成本，jsdom 下可测。
 */

type LangLoader = () => Promise<{ default: LanguageRegistration | LanguageRegistration[] }>

const LANG_LOADERS: Record<string, LangLoader> = {
  bash: () => import('shiki/langs/bash.mjs'),
  css: () => import('shiki/langs/css.mjs'),
  go: () => import('shiki/langs/go.mjs'),
  html: () => import('shiki/langs/html.mjs'),
  java: () => import('shiki/langs/java.mjs'),
  javascript: () => import('shiki/langs/javascript.mjs'),
  json: () => import('shiki/langs/json.mjs'),
  jsx: () => import('shiki/langs/jsx.mjs'),
  markdown: () => import('shiki/langs/markdown.mjs'),
  python: () => import('shiki/langs/python.mjs'),
  sql: () => import('shiki/langs/sql.mjs'),
  tsx: () => import('shiki/langs/tsx.mjs'),
  typescript: () => import('shiki/langs/typescript.mjs'),
  yaml: () => import('shiki/langs/yaml.mjs'),
}

/** 代码栅栏常见别名 -> 规范语言名。 */
const LANG_ALIASES: Record<string, string> = {
  js: 'javascript',
  ts: 'typescript',
  py: 'python',
  sh: 'bash',
  shell: 'bash',
  yml: 'yaml',
  md: 'markdown',
}

/** 超过该长度的代码块跳过高亮，直接纯文本（超长内容保护）。 */
const MAX_HIGHLIGHT_CHARS = 12_000

let highlighterPromise: Promise<HighlighterCore> | null = null
const loadedLanguages = new Set<string>()

function getHighlighter(): Promise<HighlighterCore> {
  highlighterPromise ??= (async () => {
    const [{ createHighlighterCore }, { createJavaScriptRegexEngine }, themeModule] =
      await Promise.all([
        import('shiki/core'),
        import('shiki/engine/javascript'),
        import('shiki/themes/github-light.mjs'),
      ])
    return createHighlighterCore({
      themes: [themeModule.default],
      langs: [],
      engine: createJavaScriptRegexEngine(),
    })
  })()
  return highlighterPromise
}

/**
 * 返回高亮后的 HTML（shiki 生成的 token span，不含任何输入 HTML，可安全注入）。
 * 不可高亮（未知语言/超长/失败）时返回 null，调用方退回纯文本 <pre>。
 */
export async function highlightCode(code: string, language: string): Promise<string | null> {
  const lang = LANG_ALIASES[language] ?? language
  const loader = LANG_LOADERS[lang]
  if (!loader || code.length > MAX_HIGHLIGHT_CHARS) return null
  try {
    const highlighter = await getHighlighter()
    if (!loadedLanguages.has(lang)) {
      const registration = (await loader()).default
      await highlighter.loadLanguage(
        ...(Array.isArray(registration) ? registration : [registration]),
      )
      loadedLanguages.add(lang)
    }
    // shiki 内联背景色与界面 token 冲突，剥掉背景、保留语法配色。
    return highlighter
      .codeToHtml(code, { lang, theme: 'github-light' })
      .replace(/background-color:[^;"]+;?/g, '')
  } catch {
    return null
  }
}
