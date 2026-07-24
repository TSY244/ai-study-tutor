/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { JobStreamData } from './jobStreamData';
import type { PersistedJobStreamEventEvent } from './persistedJobStreamEventEvent';

export interface PersistedJobStreamEvent {
  /** @minimum 1 */
  id: number;
  event: PersistedJobStreamEventEvent;
  data: JobStreamData;
}
