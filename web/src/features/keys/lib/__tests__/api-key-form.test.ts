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

import type { TFunction } from 'i18next'

import type { ApiKey } from '../../types'
import {
  API_KEY_FORM_DEFAULT_VALUES,
  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../api-key-form'

const apiKey: ApiKey = {
  id: 1,
  name: 'ratio-key',
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
  max_channel_ratio: 10,
}

describe('API key channel ratio form mapping', () => {
  test('uses 10 as the default maximum channel ratio', () => {
    const payload = transformFormDataToPayload(API_KEY_FORM_DEFAULT_VALUES)

    assert.equal(payload.max_channel_ratio, 10)
  })

  test('preserves a custom maximum channel ratio through form conversion', () => {
    const formValues = transformApiKeyToFormDefaults({
      ...apiKey,
      max_channel_ratio: 1.5,
    })
    const payload = transformFormDataToPayload(formValues)

    assert.equal(formValues.max_channel_ratio, 1.5)
    assert.equal(payload.max_channel_ratio, 1.5)
  })

  test('rejects an empty or non-positive maximum channel ratio', () => {
    const t = ((key: string) => key) as TFunction
    const schema = getApiKeyFormSchema(t)

    for (const maxChannelRatio of [undefined, 0, -1]) {
      const result = schema.safeParse({
        ...API_KEY_FORM_DEFAULT_VALUES,
        max_channel_ratio: maxChannelRatio,
      })

      assert.equal(result.success, false)
      if (!result.success) {
        assert.equal(
          result.error.issues.find(
            (issue) => issue.path[0] === 'max_channel_ratio'
          )?.message,
          'Channel ratio must be greater than 0'
        )
      }
    }
  })
})
