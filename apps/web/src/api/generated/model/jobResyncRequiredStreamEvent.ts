/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { JobResyncRequired } from './jobResyncRequired';

export interface JobResyncRequiredStreamEvent {
  /**
     * 当前持久化 Job Event 高水位；该 SSE 事件自身不写入 job_events。
     * @minimum 0
     */
  id: number;
  event: 'stream.resync_required';
  data: JobResyncRequired;
}
