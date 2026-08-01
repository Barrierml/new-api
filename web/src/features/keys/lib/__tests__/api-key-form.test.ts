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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { ApiKey } from '../../types'
import {
  API_KEY_FORM_DEFAULT_VALUES,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../api-key-form'

const apiKey: ApiKey = {
  id: 1,
  name: 'billing-key',
  key: 'sk-masked',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: true,
  expired_time: -1,
  created_time: 1,
  accessed_time: 1,
  group: '',
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
  billing_preference: '',
}

describe('API key billing preference form mapping', () => {
  test('maps the inherited form selection to an empty API value', () => {
    const payload = transformFormDataToPayload({
      ...API_KEY_FORM_DEFAULT_VALUES,
      billing_preference: null,
    })

    assert.equal(payload.billing_preference, '')
  })

  test('preserves an explicit key preference through form conversion', () => {
    const formValues = transformApiKeyToFormDefaults({
      ...apiKey,
      billing_preference: 'wallet_only',
    })
    const payload = transformFormDataToPayload(formValues)

    assert.equal(formValues.billing_preference, 'wallet_only')
    assert.equal(payload.billing_preference, 'wallet_only')
  })
})
