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
import { Blocks, BookOpen, House, Rocket, Terminal } from 'lucide-react'

import type { GroupMeta } from '../lib/docs'

/** GROUP_META.icon 字符串 → lucide 组件,着陆页卡片和侧边栏共用。 */
export const GROUP_ICONS: Record<
  GroupMeta['icon'],
  typeof Rocket
> = {
  rocket: Rocket,
  terminal: Terminal,
  'book-open': BookOpen,
  blocks: Blocks,
  house: House,
}
