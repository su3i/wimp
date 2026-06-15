import { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { projectService } from '@/services/project.service'

interface CreateProjectModalProps {
  open: boolean
  onClose: () => void
  onCreated: () => void
}

function slugify(str: string) {
  return str
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

export function CreateProjectModal({ open, onClose, onCreated }: CreateProjectModalProps) {
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const slug = slugify(name)

  useEffect(() => {
    if (!open) {
      setName('')
      setError('')
    }
  }, [open])

  async function handleSubmit() {
    setError('')
    setLoading(true)
    try {
      await projectService.create({ name, key: slug })
      onCreated()
      onClose()
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Failed to create project. Please try again.'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New project"
      description="Create a project to manage Windows infrastructure."
    >
      <form onSubmit={e => { e.preventDefault(); void handleSubmit() }} className="space-y-4">
        <Input
          label="Project Name"
          placeholder="Production Servers"
          value={name}
          onChange={e => setName(e.target.value)}
          autoFocus
          required
        />

        {/* Slug preview */}
        <div className="space-y-1.5">
          <label className="block text-xs font-medium text-ink-dim">Project Key</label>
          <div className="rounded-md border border-rim bg-canvas px-3 py-2 font-mono text-xs text-ink-faint select-none">
            {slug || <span className="opacity-40">auto-generated from name</span>}
          </div>
        </div>

        {error && (
          <p className="rounded border border-danger/20 bg-danger/5 px-3 py-2 text-xs text-danger">
            {error}
          </p>
        )}

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="outline" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={loading} disabled={!slug}>
            Create project
          </Button>
        </div>
      </form>
    </Modal>
  )
}
