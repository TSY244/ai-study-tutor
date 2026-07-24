/** 极简 className 合并：过滤 falsy 后拼接。刻意不引入 clsx/tailwind-merge。 */
export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}
