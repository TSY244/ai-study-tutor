/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { HeartbeatStreamData } from './heartbeatStreamData';

/**
 * 非持久化心跳；线上 SSE 帧不包含 id 行，不改变 Last-Event-ID。
 */
export interface HeartbeatStreamEvent {
  event: 'heartbeat';
  data: HeartbeatStreamData;
}
