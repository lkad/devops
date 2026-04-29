import { useState, useCallback } from 'react'
import { useToast } from '@/components/ui/Toast'

interface UseApiOptions<T> {
  onSuccess?: (data: T) => void
  onError?: (error: Error) => void
}

export function useApi<TArgs, TResult>(
  apiCall: (args: TArgs) => Promise<TResult>,
  options?: UseApiOptions<TResult>
) {
  const [data, setData] = useState<TResult | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const { addToast } = useToast()

  const execute = useCallback(
    async (args: TArgs) => {
      setIsLoading(true)
      setError(null)

      try {
        const result = await apiCall(args)
        setData(result)
        options?.onSuccess?.(result)
        return result
      } catch (err) {
        const error = err instanceof Error ? err : new Error('An error occurred')
        setError(error)
        options?.onError?.(error)
        addToast({
          type: 'error',
          title: 'Error',
          message: error.message || 'An error occurred',
        })
        throw error
      } finally {
        setIsLoading(false)
      }
    },
    [apiCall, options, addToast]
  )

  const reset = useCallback(() => {
    setData(null)
    setError(null)
    setIsLoading(false)
  }, [])

  return {
    data,
    isLoading,
    error,
    execute,
    reset,
  }
}

export function useApiWithoutArgs<TResult>(
  apiCall: () => Promise<TResult>,
  options?: UseApiOptions<TResult>
) {
  const [data, setData] = useState<TResult | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const { addToast } = useToast()

  const execute = useCallback(async () => {
    setIsLoading(true)
    setError(null)

    try {
      const result = await apiCall()
      setData(result)
      options?.onSuccess?.(result)
      return result
    } catch (err) {
      const error = err instanceof Error ? err : new Error('An error occurred')
      setError(error)
      options?.onError?.(error)
      addToast({
        type: 'error',
        title: 'Error',
        message: error.message || 'An error occurred',
      })
      throw error
    } finally {
      setIsLoading(false)
    }
  }, [apiCall, options, addToast])

  const reset = useCallback(() => {
    setData(null)
    setError(null)
    setIsLoading(false)
  }, [])

  return {
    data,
    isLoading,
    error,
    execute,
    reset,
  }
}