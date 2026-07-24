/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { HeartbeatStreamEvent } from './heartbeatStreamEvent';
import type { JobResyncRequiredStreamEvent } from './jobResyncRequiredStreamEvent';
import type { PersistedJobStreamEvent } from './persistedJobStreamEvent';

export type JobStreamEvent = PersistedJobStreamEvent | JobResyncRequiredStreamEvent | HeartbeatStreamEvent;
