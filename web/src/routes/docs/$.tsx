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
import { createFileRoute } from '@tanstack/react-router'

import { DocsPage } from '@/features/docs'

// Splat 路由:匹配 /docs/<任意路径>(如 /docs/cli/00-quickstart)。
// /docs 本身由同目录的 index.tsx 处理;两者共用同一个 DocsPage 组件,
// DocsPage 内部用 useParams 取 _splat(首页时为空串)。
export const Route = createFileRoute('/docs/$')({
  component: DocsPage,
})
