/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { CursorParameter } from './cursorParameter';
import type { JobKind } from './jobKind';
import type { JobStatus } from './jobStatus';
import type { LimitParameter } from './limitParameter';

export type ListJobsParams = {
cursor?: CursorParameter;
/**
 * @minimum 1
 * @maximum 100
 */
limit?: LimitParameter;
status?: JobStatus;
/**
 * 为 true 时只返回 queued/running/validating/cancel_requested；不能与 status 同时使用。
 */
active?: boolean;
kind?: JobKind;
aggregate_type?: string;
aggregate_id?: string;
};
