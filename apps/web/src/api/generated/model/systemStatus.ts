/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { ComponentStatus } from './componentStatus';
import type { MaintenanceStatus } from './maintenanceStatus';
import type { SystemStatusStatus } from './systemStatusStatus';

export interface SystemStatus {
  status: SystemStatusStatus;
  version: string;
  maintenance: MaintenanceStatus;
  database: ComponentStatus;
  worker: ComponentStatus;
  claude: ComponentStatus;
  storage: ComponentStatus;
}
