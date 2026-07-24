/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { FieldError } from './fieldError';

export interface Problem {
  type: string;
  title: string;
  /**
     * @minimum 400
     * @maximum 599
     */
  status: number;
  detail: string;
  instance?: string;
  code: string;
  request_id: string;
  errors?: FieldError[];
  [key: string]: unknown;
 }
