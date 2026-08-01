import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftRight, Loader2, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import {
  createModelMappingGroup,
  deleteModelMappingGroup,
  getModelMappingGroups,
  setModelMappingGroupStatus,
  updateModelMappingGroup,
} from './api'
import type { ModelMappingGroup } from './types'

const QUERY_KEY = ['model-mapping-groups']

export function ModelMappingGroups() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorGroup, setEditorGroup] = useState<ModelMappingGroup | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [deletingGroup, setDeletingGroup] = useState<ModelMappingGroup | null>(null)

  const groupsQuery = useQuery({
    queryKey: QUERY_KEY,
    queryFn: async () => {
      const r = await getModelMappingGroups()
      if (!r.success) throw new Error(r.message || 'failed')
      return r.data ?? []
    },
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: QUERY_KEY })

  const statusMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      setModelMappingGroupStatus(id, enabled),
    onSuccess: (r, vars) => {
      if (!r.success) {
        toast.error(r.message || t('Operation failed'))
        return
      }
      toast.success(
        vars.enabled ? t('Group enabled, traffic switched') : t('Group disabled, traffic restored')
      )
      void invalidate()
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteModelMappingGroup(id),
    onSuccess: (r) => {
      if (!r.success) {
        toast.error(r.message || t('Operation failed'))
        return
      }
      toast.success(t('Deleted'))
      setDeletingGroup(null)
      void invalidate()
    },
    onError: (e: Error) => toast.error(e.message),
  })

  // 启用组置顶,便于故障时一眼看到当前生效的切换
  const groups = useMemo(() => {
    const list = groupsQuery.data ?? []
    return [...list].sort((a, b) => Number(b.enabled) - Number(a.enabled))
  }, [groupsQuery.data])

  const conflicts = useMemo(() => findEnabledConflicts(groups), [groups])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Model Mappings')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void groupsQuery.refetch()}
          disabled={groupsQuery.isFetching}
        >
          <RefreshCw className={cn('size-3.5', groupsQuery.isFetching && 'animate-spin')} />
          {t('Refresh')}
        </Button>
        <Button
          size='sm'
          onClick={() => {
            setEditorGroup(null)
            setEditorOpen(true)
          }}
        >
          <Plus className='size-3.5' />
          {t('New Group')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <p className='text-sm text-muted-foreground'>
            {t(
              'Globally redirect a model to another before channel selection — billing and logs stay on the original model. Enable a group to switch instantly; disable to roll back.'
            )}
          </p>
          {conflicts.length > 0 && (
            <div className='rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive'>
              {t('Enabled groups conflict on source model')}: {conflicts.join(', ')}
            </div>
          )}
          {groupsQuery.isLoading && (
            <div className='flex items-center gap-2 text-muted-foreground'>
              <Loader2 className='size-4 animate-spin' /> {t('Loading...')}
            </div>
          )}
          {groupsQuery.isError && (
            <div className='text-destructive'>{t('Failed to load')}</div>
          )}
          {groupsQuery.isSuccess && groups.length === 0 && (
            <div className='text-sm text-muted-foreground'>
              {t('No mapping groups yet. Create one to prepare an instant failover switch.')}
            </div>
          )}
          {groups.map((g) => (
            <GroupCard
              key={g.id}
              group={g}
              toggling={statusMutation.isPending && statusMutation.variables?.id === g.id}
              onToggle={(enabled) => statusMutation.mutate({ id: g.id, enabled })}
              onEdit={() => {
                setEditorGroup(g)
                setEditorOpen(true)
              }}
              onDelete={() => setDeletingGroup(g)}
            />
          ))}
        </div>
      </SectionPageLayout.Content>

      <GroupEditorDialog
        open={editorOpen}
        group={editorGroup}
        onClose={() => setEditorOpen(false)}
        onSaved={() => {
          setEditorOpen(false)
          void invalidate()
        }}
      />

      <AlertDialog open={!!deletingGroup} onOpenChange={(open) => !open && setDeletingGroup(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete mapping group?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deletingGroup?.enabled
                ? t('This group is currently enabled — deleting it restores direct routing immediately.')
                : t('This only removes the preset group; channel configs are untouched.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deletingGroup && deleteMutation.mutate(deletingGroup.id)}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SectionPageLayout>
  )
}

// 后端已兜底 400,这里做前端预警:启用组间源模型重复。
function findEnabledConflicts(groups: ModelMappingGroup[]): string[] {
  const seen = new Map<string, string>()
  const dupes = new Set<string>()
  for (const g of groups) {
    if (!g.enabled) continue
    for (const k of Object.keys(g.mappings)) {
      if (seen.has(k)) {
        dupes.add(k)
      } else {
        seen.set(k, g.name)
      }
    }
  }
  return [...dupes]
}

function GroupCard({
  group,
  toggling,
  onToggle,
  onEdit,
  onDelete,
}: {
  group: ModelMappingGroup
  toggling: boolean
  onToggle: (enabled: boolean) => void
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  const entries = Object.entries(group.mappings)
  return (
    <Card className={cn(group.enabled && 'border-emerald-500/60')}>
      <CardHeader className='flex flex-row items-center justify-between gap-3 space-y-0'>
        <div className='flex items-center gap-2'>
          <CardTitle className='text-base'>{group.name}</CardTitle>
          {group.enabled ? (
            <Badge className='bg-emerald-600'>{t('Active')}</Badge>
          ) : (
            <Badge variant='secondary'>{t('Standby')}</Badge>
          )}
        </div>
        <div className='flex items-center gap-2'>
          <span className='text-xs text-muted-foreground'>
            {group.enabled ? t('Enabled') : t('Disabled')}
          </span>
          <Switch
            checked={group.enabled}
            disabled={toggling}
            onCheckedChange={onToggle}
            aria-label={t('Toggle group')}
          />
          <Button variant='ghost' size='icon' onClick={onEdit}>
            <Pencil className='size-4' />
          </Button>
          <Button variant='ghost' size='icon' onClick={onDelete}>
            <Trash2 className='size-4 text-destructive' />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Source Model')}</TableHead>
              <TableHead className='w-10' />
              <TableHead>{t('Target Model')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map(([from, to]) => (
              <TableRow key={from}>
                <TableCell className='font-mono text-sm'>{from}</TableCell>
                <TableCell>
                  <ArrowLeftRight className='size-3.5 text-muted-foreground' />
                </TableCell>
                <TableCell className='font-mono text-sm'>{to}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <div className='mt-2 text-xs text-muted-foreground'>
          {t('Updated')}: {group.updated_at ? new Date(group.updated_at * 1000).toLocaleString() : '-'}
        </div>
      </CardContent>
    </Card>
  )
}

interface MappingRow {
  from: string
  to: string
}

function GroupEditorDialog({
  open,
  group,
  onClose,
  onSaved,
}: {
  open: boolean
  group: ModelMappingGroup | null
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [rows, setRows] = useState<MappingRow[]>([{ from: '', to: '' }])
  const [lastOpen, setLastOpen] = useState(false)

  // 打开时初始化表单(用 open 上升沿触发,避免受控输入被重置)
  if (open && !lastOpen) {
    setLastOpen(true)
    setName(group?.name ?? '')
    const entries = Object.entries(group?.mappings ?? {})
    setRows(entries.length > 0 ? entries.map(([from, to]) => ({ from, to })) : [{ from: '', to: '' }])
  } else if (!open && lastOpen) {
    setLastOpen(false)
  }

  const saveMutation = useMutation({
    mutationFn: async () => {
      const mappings: Record<string, string> = {}
      for (const r of rows) {
        const from = r.from.trim()
        const to = r.to.trim()
        if (from) mappings[from] = to
      }
      const payload = { name: name.trim(), mappings }
      const r = group
        ? await updateModelMappingGroup(group.id, payload)
        : await createModelMappingGroup(payload)
      if (!r.success) throw new Error(r.message || t('Operation failed'))
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      onSaved()
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const rowsValid = rows.every((r) => r.from.trim() && r.to.trim())
  const canSave = name.trim() && rows.length > 0 && rowsValid && !saveMutation.isPending

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className='max-w-lg'>
        <DialogHeader>
          <DialogTitle>{group ? t('Edit Group') : t('New Group')}</DialogTitle>
        </DialogHeader>
        <div className='flex flex-col gap-3'>
          <Input
            placeholder={t('Group name, e.g. gpt-5.2 failover')}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          {rows.map((row, i) => (
            <div key={i} className='flex items-center gap-2'>
              <Input
                className='font-mono'
                placeholder={t('source model')}
                value={row.from}
                onChange={(e) =>
                  setRows(rows.map((r, j) => (j === i ? { ...r, from: e.target.value } : r)))
                }
              />
              <ArrowLeftRight className='size-4 shrink-0 text-muted-foreground' />
              <Input
                className='font-mono'
                placeholder={t('target model')}
                value={row.to}
                onChange={(e) =>
                  setRows(rows.map((r, j) => (j === i ? { ...r, to: e.target.value } : r)))
                }
              />
              <Button
                variant='ghost'
                size='icon'
                disabled={rows.length <= 1}
                onClick={() => setRows(rows.filter((_, j) => j !== i))}
              >
                <Trash2 className='size-4' />
              </Button>
            </div>
          ))}
          <Button
            variant='outline'
            size='sm'
            className='self-start'
            onClick={() => setRows([...rows, { from: '', to: '' }])}
          >
            <Plus className='size-3.5' />
            {t('Add mapping')}
          </Button>
          <p className='text-xs text-muted-foreground'>
            {t('New groups start disabled. Billing and logs always follow the source model.')}
          </p>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={onClose}>
            {t('Cancel')}
          </Button>
          <Button disabled={!canSave} onClick={() => saveMutation.mutate()}>
            {saveMutation.isPending && <Loader2 className='size-3.5 animate-spin' />}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
