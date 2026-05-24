export interface SearchResult {
  content?: string
  text?: string
  [key: string]: unknown
}

export interface TargetState {
  job: string
  instance: string
  up: boolean
}

export interface MetricsData {
  status: string
  cpu?: number
  memory?: number
  mem_total_gb?: number
  mem_used_gb?: number
  disk?: number
  disk_io_util?: number
  inode_use?: number
  swap_use?: number
  load1m?: number
  load5m?: number
  load15m?: number
  network_rx?: number
  network_tx?: number
  tcp_tw?: number
  oom_kills_1h?: number
  up_count: number
  down_count: number
  targets?: TargetState[]
  error?: string
}
