import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Settings, RefreshCw, AlertCircle, CheckCircle } from 'lucide-react'
import { PageContainer } from '@/components/layout'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { ConfigPushForm } from './ConfigPushForm'
import { physicalHostsApi, type HostConfig } from '@/api/endpoints/physicalHosts'

export function HostConfig() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const [isPushFormOpen, setIsPushFormOpen] = useState(false)

  const { data: configData, isLoading, error } = useQuery({
    queryKey: ['physical-hosts', id, 'config'],
    queryFn: () => physicalHostsApi.getConfig(id!),
    enabled: !!id,
  })

  const pushMutation = useMutation({
    mutationFn: (content: string) => physicalHostsApi.pushConfig(id!, content),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['physical-hosts', id, 'config'] })
      setIsPushFormOpen(false)
    },
  })

  const currentConfig = configData?.content

  return (
    <PageContainer
      title="Host Configuration"
      description={`Configuration for host ${id}`}
      actions={
        <Button
          variant="primary"
          size="sm"
          onClick={() => setIsPushFormOpen(true)}
          leftIcon={<RefreshCw className="w-4 h-4" />}
        >
          Push Config
        </Button>
      }
    >
      <div className="space-y-6">
        {isLoading ? (
          <Card>
            <div className="flex items-center justify-center py-12">
              <p className="text-text-secondary">Loading configuration...</p>
            </div>
          </Card>
        ) : error ? (
          <Card>
            <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
              <AlertCircle className="w-12 h-12 mb-4 opacity-50" />
              <p>Failed to load configuration</p>
              <p className="text-sm mt-1">No configuration found for this host</p>
            </div>
          </Card>
        ) : currentConfig ? (
          <Card>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Settings className="w-5 h-5 text-text-secondary" />
                <h2 className="text-lg font-semibold text-text">Current Configuration</h2>
              </div>
              <Badge variant="success">
                <CheckCircle className="w-3 h-3" />
                Configured
              </Badge>
            </div>
            <pre className="bg-surface p-4 rounded-lg overflow-x-auto text-sm font-mono text-text">
              {currentConfig}
            </pre>
          </Card>
        ) : (
          <Card>
            <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
              <AlertCircle className="w-12 h-12 mb-4 opacity-50" />
              <p>No configuration found for this host</p>
              <p className="text-sm mt-1">Click "Push Config" to add a configuration</p>
            </div>
          </Card>
        )}
      </div>

      <ConfigPushForm
        isOpen={isPushFormOpen}
        onClose={() => setIsPushFormOpen(false)}
        onSubmit={pushMutation.mutate}
        currentConfig={currentConfig}
        isLoading={pushMutation.isPending}
      />
    </PageContainer>
  )
}