/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 应用标题（非敏感） */
  readonly VITE_APP_TITLE?: string
  /** 开发时启用 MSW mock（'true'/'false'） */
  readonly VITE_ENABLE_MSW?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
