import useSWR from 'swr'
import { api } from '@/lib/api'
import type { StatusResponse } from '@/types/api'

export function useStatus(enabled: boolean = true) {
  const { data, error, isLoading, mutate } = useSWR<StatusResponse>(
    enabled ? '/status' : null,
    () => api.getStatus(),
    {
      refreshInterval: enabled ? 5000 : 0,
      revalidateOnFocus: enabled,
      revalidateOnReconnect: enabled,
    }
  )

  // Debug logging to help troubleshoot motion/sound timestamp issues
  if (data) {
    console.log('API Status Response:', data)
    data.babies?.forEach((baby, index) => {
      console.log(`Baby ${index} (${baby.name}):`, baby)
    })
  }
  
  if (error) {
    console.error('Status API Error:', error)
  }

  return {
    status: data,
    babies: data?.babies ?? [],
    isLoading,
    isError: !!error,
    error,
    refresh: mutate,
  }
}