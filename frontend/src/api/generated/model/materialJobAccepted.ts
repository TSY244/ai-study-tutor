/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { Job } from './job';
import type { Material } from './material';

export interface MaterialJobAccepted {
  material: Material;
  job: Job;
}
