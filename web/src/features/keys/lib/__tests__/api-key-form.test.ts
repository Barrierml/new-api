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
import type { TFunction } from 'i18next'

  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../api-key-form'

const apiKey: ApiKey = {
  id: 1,
  name: 'billing-key',
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
    const payload = transformFormDataToPayload(formValues)
    assert.equal(formValues.billing_preference, 'wallet_only')
    assert.equal(payload.billing_preference, 'wallet_only')
  max_channel_ratio: 0,
describe('API key channel ratio form mapping', () => {
  test('defaults to 0 (unlimited) as the maximum channel ratio', () => {
    const payload = transformFormDataToPayload(API_KEY_FORM_DEFAULT_VALUES)
    assert.equal(payload.max_channel_ratio, 0)
  test('preserves a custom maximum channel ratio through form conversion', () => {
      max_channel_ratio: 1.5,
    assert.equal(formValues.max_channel_ratio, 1.5)
    assert.equal(payload.max_channel_ratio, 1.5)
  test('rejects an empty or negative maximum channel ratio', () => {
    const t = ((key: string) => key) as TFunction
    const schema = getApiKeyFormSchema(t)
    for (const maxChannelRatio of [undefined, -1]) {
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
          'Channel ratio must be 0 (unlimited) or greater than 0'
        )
      }
    }
  })

  test('accepts 0 as unlimited maximum channel ratio', () => {
    const t = ((key: string) => key) as TFunction
    const schema = getApiKeyFormSchema(t)
    const result = schema.safeParse({
      ...API_KEY_FORM_DEFAULT_VALUES,
      max_channel_ratio: 0,
    })
    assert.equal(result.success, true)
  })
})
