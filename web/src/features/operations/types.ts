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
/**
 * Types for the admin operations overview page
 */

export interface OperationsCards {
  today_quota: number
  today_prompt_tokens: number
  today_completion_tokens: number
  active_users_today: number
  total_users: number
  active_connections: number
}

export interface OperationsWindow {
  limit_usd: number
  used_usd: number
  percent: number
  window_end: number
}

export interface OperationsActiveUser {
  user_id: number
  username: string
  display_name: string
  plan_title: string
  today_quota: number
  today_prompt_tokens: number
  today_completion_tokens: number
  request_count: number
  last_used_at: number
  wallet_quota: number
  window_5h: OperationsWindow | null
  weekly: OperationsWindow | null
}

export interface OperationsLogItem {
  id: number
  created_at: number
  type: number
  username: string
  user_display_name?: string
  user_email?: string
  model_name: string
  channel_name?: string
  content: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
}

export interface OperationsOverview {
  generated_at: number
  day_start: number
  cards: OperationsCards
  users: OperationsActiveUser[] | null
  error_logs: OperationsLogItem[] | null
  consume_logs: OperationsLogItem[] | null
}
