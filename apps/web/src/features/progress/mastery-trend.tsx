import type { ProgressPoint } from '@/api/generated/model'
import { formatDateTime } from '@/lib/format'

const WIDTH = 260
const HEIGHT = 64
const PAD = 8

/**
 * 掌握度趋势（纯展示）。掌握度由后端按 formula_version 计算，前端不做
 * 任何重算/插值/外推，只把 trend 中的点按时间轴画出来并格式化为百分比。
 * 无可动效，reduced motion 无需处理；屏幕阅读器走 sr-only 数据列表。
 */
export function MasteryTrend({ trend, label }: { trend: ProgressPoint[]; label: string }) {
  const points = [...trend].sort((a, b) => a.recorded_at.localeCompare(b.recorded_at))

  if (points.length < 2) {
    return <p className="text-sm text-text-muted">完成更多测验后，这里会显示掌握度趋势。</p>
  }

  const first = points[0]
  const last = points[points.length - 1]
  const start = Date.parse(first.recorded_at)
  const end = Date.parse(last.recorded_at)
  const span = end - start

  const coords = points.map((point, index) => {
    const x =
      span > 0
        ? PAD + ((Date.parse(point.recorded_at) - start) / span) * (WIDTH - PAD * 2)
        : PAD + (index / (points.length - 1)) * (WIDTH - PAD * 2)
    const y = HEIGHT - PAD - point.mastery * (HEIGHT - PAD * 2)
    return { x, y }
  })

  const startPct = Math.round(first.mastery * 100)
  const endPct = Math.round(last.mastery * 100)

  return (
    <figure aria-label={`${label}掌握度趋势`}>
      <svg
        role="img"
        aria-label={`${label}掌握度从 ${startPct}% 变化到 ${endPct}%，共 ${points.length} 次记录`}
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="h-16 w-full"
        preserveAspectRatio="none"
      >
        {/* 0% 与 100% 参考线 */}
        <line
          x1={0}
          x2={WIDTH}
          y1={HEIGHT - PAD}
          y2={HEIGHT - PAD}
          stroke="var(--color-border)"
          strokeWidth={1}
          vectorEffect="non-scaling-stroke"
        />
        <line
          x1={0}
          x2={WIDTH}
          y1={PAD}
          y2={PAD}
          stroke="var(--color-border)"
          strokeWidth={1}
          strokeDasharray="3 3"
          vectorEffect="non-scaling-stroke"
        />
        <polyline
          points={coords.map((coord) => `${coord.x},${coord.y}`).join(' ')}
          fill="none"
          stroke="var(--color-accent)"
          strokeWidth={2}
          strokeLinejoin="round"
          strokeLinecap="round"
          vectorEffect="non-scaling-stroke"
        />
        {coords.map((coord, index) => (
          <circle
            key={index}
            cx={coord.x}
            cy={coord.y}
            r={2.5}
            fill="var(--color-accent)"
            vectorEffect="non-scaling-stroke"
          />
        ))}
      </svg>
      <figcaption className="sr-only">
        <ol>
          {points.map((point) => (
            <li key={point.recorded_at}>
              {formatDateTime(point.recorded_at)}：掌握度 {Math.round(point.mastery * 100)}%
            </li>
          ))}
        </ol>
      </figcaption>
    </figure>
  )
}
