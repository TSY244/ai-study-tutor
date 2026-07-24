/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { RegenerationCreateKindsItem } from './regenerationCreateKindsItem';

export interface RegenerationCreate {
  /** @minItems 1 */
  kinds: RegenerationCreateKindsItem[];
  /** @maxLength 1000 */
  reason?: string;
}
