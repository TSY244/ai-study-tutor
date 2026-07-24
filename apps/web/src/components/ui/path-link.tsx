import { Link } from '@tanstack/react-router'
import type { ComponentType, ReactNode } from 'react'

/**
 * 接受运行时字符串路径的 Link。面包屑等场景的跳转目标来自路由匹配结果
 * （一定是应用内真实路径），但 TanStack Router 的字面量类型无法表达
 * “任意已注册路径”，这里做一次受控放宽。
 */
export const PathLink = Link as unknown as ComponentType<{
  to: string
  children?: ReactNode
  className?: string
  'aria-label'?: string
}>
