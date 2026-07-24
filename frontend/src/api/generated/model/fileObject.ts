/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface FileObject {
  id: string;
  /** @nullable */
  display_name?: string | null;
  media_type: string;
  /** @minimum 0 */
  size_bytes: number;
  /** @pattern ^[a-f0-9]{64}$ */
  sha256: string;
  download_url: string;
  created_at: string;
}
