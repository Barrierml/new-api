/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type SystemInstanceStatus = 'online' | 'stale'

export type SystemInstanceInfo = {
  schema_version?: number
  node?: {
    name?: string
    source?: string
    manually_configured?: boolean
    should_configure_manually?: boolean
    [key: string]: unknown
  }
  role?: {
    is_master?: boolean
    [key: string]: unknown
  }
  runtime?: {
    version?: string
    goos?: string
    goarch?: string
    started_at?: number
    [key: string]: unknown
  }
  host?: {
    hostname?: string
    [key: string]: unknown
  }
  resources?: {
    cpu?: {
      usage_percent?: number
      [key: string]: unknown
    }
    memory?: {
      usage_percent?: number
      [key: string]: unknown
    }
    storage?: {
      total_bytes?: number
      used_bytes?: number
      free_bytes?: number
      used_percent?: number
      [key: string]: unknown
    }
    [key: string]: unknown
  }
  [key: string]: unknown
}

export type SystemInstance = {
  node_name: string
  status: SystemInstanceStatus
  stale_after_seconds: number
  started_at: number
  last_seen_at: number
  info?: SystemInstanceInfo
}

export type SystemInstanceListResponse = {
  success: boolean
  message: string
  data?: SystemInstance[]
}

export type UpstreamQuotaStatus =
  | 'available'
  | 'limited'
  | 'exhausted'
  | 'unknown'
  | 'error'
  | 'deferred'

export type UpstreamQuotaWindow = {
  key?: string
  kind?: string
  label?: string
  unit?: string
  limit?: number
  used?: number
  remaining?: number
  remaining_pct?: number
  reset_at?: number
  duration_seconds?: number
}

export type UpstreamQuotaChannel = {
  id: number
  name: string
  priority: number
  status: number
}

export type UpstreamQuotaEntity = {
  entity_id: string
  display_name: string
  provider: string
  status: UpstreamQuotaStatus
  status_message: string
  account_ids?: number[]
  group_ids?: number[]
  channel_ids?: number[]
  channels?: UpstreamQuotaChannel[]
  windows?: UpstreamQuotaWindow[]
  fetched_at?: string
  available_account_count?: number
  stale: boolean
}

export type UpstreamQuotaCounts = Record<UpstreamQuotaStatus | 'stale', number>

export type UpstreamQuotaDashboard = {
  generated_at: string
  entity_count: number
  counts: UpstreamQuotaCounts
  entities: UpstreamQuotaEntity[]
  stale: boolean
}

export type UpstreamQuotaDashboardResponse = {
  success: boolean
  message: string
  data?: UpstreamQuotaDashboard
}

export type SystemInstanceDeleteResponse = {
  success: boolean
  message: string
  data?: {
    deleted_count: number
  }
}
