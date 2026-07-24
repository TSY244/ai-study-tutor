/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Attempt } from './attempt';
import type { Job } from './job';

export interface AttemptSubmitAccepted {
  attempt: Attempt;
  job: Job;
}
