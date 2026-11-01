/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { UploadMaterialBodyTranscriptionMode } from './uploadMaterialBodyTranscriptionMode';

export type UploadMaterialBody = {
  file: Blob;
  /** 音频识别语言，如 zh、en；留空时自动识别。 */
  preferred_language?: string;
  transcription_mode?: UploadMaterialBodyTranscriptionMode;
};
